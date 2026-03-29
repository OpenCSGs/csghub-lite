package server

import (
	"context"
	crand "crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	neturl "net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/x/xpty"
	"github.com/gorilla/websocket"

	"github.com/opencsgs/csghub-lite/internal/config"
)

const (
	aiAppShellDefaultCols = 120
	aiAppShellDefaultRows = 36
	aiAppShellReplayLimit = 256 * 1024
	aiAppShellIdleTimeout = 30 * time.Second
	openCodeWebProviderID = "csghub-lite"
)

var aiAppShellUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type aiAppOpenTarget struct {
	AppID       string
	DisplayName string
	Binaries    []string
}

type aiAppPreparedLaunch struct {
	Binary string
	Args   []string
	Env    []string
	Dir    string
}

type aiAppShellClientMessage struct {
	Type string `json:"type"`
	Cols int    `json:"cols,omitempty"`
	Rows int    `json:"rows,omitempty"`
}

type aiAppShellControlMessage struct {
	Type     string `json:"type"`
	Session  string `json:"session_id,omitempty"`
	AppID    string `json:"app_id,omitempty"`
	Title    string `json:"title,omitempty"`
	ModelID  string `json:"model_id,omitempty"`
	WorkDir  string `json:"work_dir,omitempty"`
	ExitCode int    `json:"exit_code,omitempty"`
	Error    string `json:"error,omitempty"`
}

type aiAppShellEvent struct {
	output []byte
	exit   *aiAppShellControlMessage
}

type aiAppShellAttach struct {
	ready   aiAppShellControlMessage
	replay  []byte
	events  chan aiAppShellEvent
	exitMsg *aiAppShellControlMessage
}

type aiAppShellSession struct {
	manager *aiAppShellManager

	id      string
	appID   string
	title   string
	modelID string
	workDir string
	cmd     *exec.Cmd
	pty     xpty.Pty

	mu          sync.Mutex
	replay      []byte
	subs        map[chan aiAppShellEvent]struct{}
	done        bool
	exitCode    int
	exitErr     string
	idleTimer   *time.Timer
	terminating bool
}

type aiAppShellManager struct {
	mu       sync.RWMutex
	sessions map[string]*aiAppShellSession
}

func newAIAppShellManager() *aiAppShellManager {
	return &aiAppShellManager{
		sessions: make(map[string]*aiAppShellSession),
	}
}

func (m *aiAppShellManager) Create(appID, title, modelID string, prepared aiAppPreparedLaunch) (*aiAppShellSession, error) {
	pty, err := xpty.NewPty(aiAppShellDefaultCols, aiAppShellDefaultRows)
	if err != nil {
		return nil, fmt.Errorf("creating terminal: %w", err)
	}

	cmd := exec.Command(prepared.Binary, prepared.Args...)
	cmd.Env = prepared.Env
	cmd.Dir = prepared.Dir

	if err := pty.Start(cmd); err != nil {
		_ = pty.Close()
		return nil, fmt.Errorf("starting %s terminal: %w", title, err)
	}

	session := &aiAppShellSession{
		manager: m,
		id:      newAIAppShellSessionID(),
		appID:   appID,
		title:   title,
		modelID: modelID,
		workDir: prepared.Dir,
		cmd:     cmd,
		pty:     pty,
		subs:    make(map[chan aiAppShellEvent]struct{}),
	}

	m.mu.Lock()
	m.sessions[session.id] = session
	m.mu.Unlock()

	session.scheduleIdleTimeout()
	session.start()
	return session, nil
}

func (m *aiAppShellManager) Get(id string) (*aiAppShellSession, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	session, ok := m.sessions[id]
	return session, ok
}

func (m *aiAppShellManager) Close(id string) bool {
	session, ok := m.Get(id)
	if !ok {
		return false
	}
	session.Terminate()
	m.remove(id)
	return true
}

func (m *aiAppShellManager) remove(id string) {
	m.mu.Lock()
	delete(m.sessions, id)
	m.mu.Unlock()
}

func (s *aiAppShellSession) start() {
	go s.streamOutput()
	go s.wait()
}

func (s *aiAppShellSession) streamOutput() {
	buf := make([]byte, 4096)
	for {
		n, err := s.pty.Read(buf)
		if n > 0 {
			chunk := append([]byte(nil), buf[:n]...)
			s.appendReplay(chunk)
			s.broadcast(aiAppShellEvent{output: chunk})
		}
		if err != nil {
			if err != io.EOF {
				// Best effort: the wait goroutine will publish the final exit state.
			}
			return
		}
	}
}

func (s *aiAppShellSession) wait() {
	err := xpty.WaitProcess(context.Background(), s.cmd)

	exitCode := 0
	if s.cmd.ProcessState != nil {
		exitCode = s.cmd.ProcessState.ExitCode()
	}
	exitErr := ""
	if err != nil {
		exitErr = err.Error()
		if exitCode == 0 {
			exitCode = 1
		}
	}

	s.mu.Lock()
	if s.done {
		s.mu.Unlock()
		return
	}
	s.done = true
	s.exitCode = exitCode
	s.exitErr = exitErr
	subscribers := s.subscribersLocked()
	if len(subscribers) == 0 {
		s.scheduleIdleTimeoutLocked()
	}
	s.mu.Unlock()

	s.broadcast(aiAppShellEvent{
		exit: &aiAppShellControlMessage{
			Type:     "exit",
			Session:  s.id,
			AppID:    s.appID,
			Title:    s.title,
			ModelID:  s.modelID,
			WorkDir:  s.workDir,
			ExitCode: exitCode,
			Error:    exitErr,
		},
	})
	_ = s.pty.Close()
}

func (s *aiAppShellSession) Attach() aiAppShellAttach {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.stopIdleTimeoutLocked()

	attach := aiAppShellAttach{
		ready: aiAppShellControlMessage{
			Type:    "ready",
			Session: s.id,
			AppID:   s.appID,
			Title:   s.title,
			ModelID: s.modelID,
			WorkDir: s.workDir,
		},
		replay: append([]byte(nil), s.replay...),
	}

	if s.done {
		attach.exitMsg = &aiAppShellControlMessage{
			Type:     "exit",
			Session:  s.id,
			AppID:    s.appID,
			Title:    s.title,
			ModelID:  s.modelID,
			WorkDir:  s.workDir,
			ExitCode: s.exitCode,
			Error:    s.exitErr,
		}
		return attach
	}

	ch := make(chan aiAppShellEvent, 256)
	s.subs[ch] = struct{}{}
	attach.events = ch
	return attach
}

func (s *aiAppShellSession) Detach(ch chan aiAppShellEvent) {
	if ch == nil {
		return
	}

	s.mu.Lock()
	if _, ok := s.subs[ch]; ok {
		delete(s.subs, ch)
		close(ch)
	}
	if len(s.subs) == 0 {
		s.scheduleIdleTimeoutLocked()
	}
	s.mu.Unlock()
}

func (s *aiAppShellSession) WriteInput(p []byte) error {
	if len(p) == 0 {
		return nil
	}
	_, err := s.pty.Write(p)
	return err
}

func (s *aiAppShellSession) Resize(cols, rows int) error {
	if cols <= 0 || rows <= 0 {
		return nil
	}
	return s.pty.Resize(cols, rows)
}

func (s *aiAppShellSession) Terminate() {
	s.mu.Lock()
	if s.terminating {
		s.mu.Unlock()
		return
	}
	s.terminating = true
	s.stopIdleTimeoutLocked()
	process := s.cmd.Process
	pty := s.pty
	s.mu.Unlock()

	if process != nil {
		_ = process.Kill()
	}
	if pty != nil {
		_ = pty.Close()
	}
}

func (s *aiAppShellSession) appendReplay(chunk []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.replay = append(s.replay, chunk...)
	if len(s.replay) > aiAppShellReplayLimit {
		s.replay = append([]byte(nil), s.replay[len(s.replay)-aiAppShellReplayLimit:]...)
	}
}

func (s *aiAppShellSession) broadcast(event aiAppShellEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for ch := range s.subs {
		select {
		case ch <- event:
		default:
			// The browser terminal runs on localhost, so a full buffer is unexpected.
			// Drop rather than blocking the PTY reader indefinitely.
		}
	}
}

func (s *aiAppShellSession) subscribersLocked() []chan aiAppShellEvent {
	subscribers := make([]chan aiAppShellEvent, 0, len(s.subs))
	for ch := range s.subs {
		subscribers = append(subscribers, ch)
	}
	return subscribers
}

func (s *aiAppShellSession) scheduleIdleTimeout() {
	s.mu.Lock()
	s.scheduleIdleTimeoutLocked()
	s.mu.Unlock()
}

func (s *aiAppShellSession) scheduleIdleTimeoutLocked() {
	if s.idleTimer != nil {
		s.idleTimer.Stop()
	}
	s.idleTimer = time.AfterFunc(aiAppShellIdleTimeout, func() {
		s.Terminate()
		s.manager.remove(s.id)
	})
}

func (s *aiAppShellSession) stopIdleTimeoutLocked() {
	if s.idleTimer != nil {
		s.idleTimer.Stop()
		s.idleTimer = nil
	}
}

func (s *Server) openAIAppShellURL(ctx context.Context, appID, requestedModel, requestedWorkDir string) (string, error) {
	if s.appShells == nil {
		s.appShells = newAIAppShellManager()
	}

	target, err := resolveAIAppOpenTarget(appID)
	if err != nil {
		return "", err
	}

	defaultModel, modelIDs, err := s.resolveAIAppLaunchModels(ctx, requestedModel)
	if err != nil {
		return "", err
	}

	prepared, err := s.prepareAIAppShellLaunch(target, defaultModel, modelIDs, requestedWorkDir)
	if err != nil {
		return "", err
	}

	session, err := s.appShells.Create(target.AppID, target.DisplayName, defaultModel, prepared)
	if err != nil {
		return "", err
	}

	u, err := neturl.Parse(s.localBaseURL())
	if err != nil {
		return "", err
	}
	u.Path = "/ai-apps/shell"
	query := u.Query()
	query.Set("session_id", session.id)
	query.Set("app_id", session.appID)
	u.RawQuery = query.Encode()
	return u.String(), nil
}

func resolveAIAppOpenTarget(appID string) (aiAppOpenTarget, error) {
	switch appID {
	case "claude-code":
		return aiAppOpenTarget{
			AppID:       "claude-code",
			DisplayName: "Claude Code",
			Binaries:    []string{"claude"},
		}, nil
	case "open-code":
		return aiAppOpenTarget{
			AppID:       "open-code",
			DisplayName: "OpenCode",
			Binaries:    []string{"opencode"},
		}, nil
	case "codex":
		return aiAppOpenTarget{
			AppID:       "codex",
			DisplayName: "Codex",
			Binaries:    []string{"codex"},
		}, nil
	default:
		return aiAppOpenTarget{}, fmt.Errorf("%s does not provide a web shell entry yet", appID)
	}
}

func (s *Server) resolveAIAppLaunchModels(ctx context.Context, requestedModel string) (string, []string, error) {
	localModels, err := s.manager.List()
	if err != nil {
		return "", nil, fmt.Errorf("listing local models: %w", err)
	}

	sort.SliceStable(localModels, func(i, j int) bool {
		left := scoreOpenClawModel(localModels[i])
		right := scoreOpenClawModel(localModels[j])
		if left != right {
			return left > right
		}
		return localModels[i].DownloadedAt.After(localModels[j].DownloadedAt)
	})

	modelIDs := make([]string, 0, len(localModels)+4)
	seen := make(map[string]struct{}, len(localModels)+4)
	defaultModel := ""
	for _, item := range localModels {
		modelID := item.FullName()
		if defaultModel == "" {
			defaultModel = modelID
		}
		modelIDs = appendUniqueModelID(modelIDs, seen, modelID)
	}

	if s.cloud != nil && strings.TrimSpace(s.cfg.Token) != "" {
		s.refreshOpenClawModelCatalog(ctx)
		if cloudModels, err := s.cloud.ListChatModels(ctx); err == nil {
			for _, item := range cloudModels {
				modelIDs = appendUniqueModelID(modelIDs, seen, item.Model)
				if defaultModel == "" {
					defaultModel = item.Model
				}
			}
		}
	}

	if defaultModel == "" {
		return "", nil, fmt.Errorf("no models were found. Pull a model first, then open the app")
	}

	requestedModel = strings.TrimSpace(requestedModel)
	if requestedModel != "" {
		if _, ok := seen[requestedModel]; !ok {
			return "", nil, fmt.Errorf("model %q is not available for AI Apps", requestedModel)
		}
		return requestedModel, modelIDs, nil
	}

	return defaultModel, modelIDs, nil
}

func appendUniqueModelID(modelIDs []string, seen map[string]struct{}, modelID string) []string {
	modelID = strings.TrimSpace(modelID)
	if modelID == "" {
		return modelIDs
	}
	if _, ok := seen[modelID]; ok {
		return modelIDs
	}
	seen[modelID] = struct{}{}
	return append(modelIDs, modelID)
}

func (s *Server) prepareAIAppShellLaunch(target aiAppOpenTarget, modelID string, modelIDs []string, requestedWorkDir string) (aiAppPreparedLaunch, error) {
	binary, err := resolveAIAppLaunchBinary(target.Binaries)
	if err != nil {
		return aiAppPreparedLaunch{}, fmt.Errorf("%s is installed, but the launch command was not found on PATH", target.DisplayName)
	}

	workingDir, err := normalizeAIAppWorkDir(requestedWorkDir)
	if err != nil {
		return aiAppPreparedLaunch{}, err
	}
	serverURL := s.localBaseURL()

	switch target.AppID {
	case "claude-code":
		return aiAppPreparedLaunch{
			Binary: binary,
			Args: []string{
				"--model", modelID,
				"--settings", claudeLaunchSettingsJSON(serverURL),
			},
			Env: envWithOverrides(map[string]string{
				"ANTHROPIC_BASE_URL":   serverURL,
				"ANTHROPIC_AUTH_TOKEN": "csghub-lite",
				"ANTHROPIC_API_KEY":    "csghub-lite",
				"CLAUDE_API_BASE_URL":  serverURL,
				"CLAUDE_API_KEY":       "csghub-lite",
			}),
			Dir: workingDir,
		}, nil
	case "open-code":
		configPath, err := writeOpenCodeWebLaunchConfig(serverURL, modelID, modelIDs)
		if err != nil {
			return aiAppPreparedLaunch{}, err
		}
		return aiAppPreparedLaunch{
			Binary: binary,
			Env: envWithOverrides(map[string]string{
				"OPENCODE_CONFIG": configPath,
			}),
			Dir: workingDir,
		}, nil
	case "codex":
		return aiAppPreparedLaunch{
			Binary: binary,
			Args: []string{
				"-c", fmt.Sprintf(`openai_base_url=%q`, strings.TrimRight(serverURL, "/")+"/v1"),
				"--model", modelID,
			},
			Env: envWithOverrides(map[string]string{
				"OPENAI_API_KEY": "csghub-lite",
			}),
			Dir: workingDir,
		}, nil
	default:
		return aiAppPreparedLaunch{}, fmt.Errorf("%s does not support web shell launch yet", target.DisplayName)
	}
}

func normalizeAIAppWorkDir(requested string) (string, error) {
	requested = strings.TrimSpace(requested)
	if requested == "" {
		dir, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("determining working directory: %w", err)
		}
		return dir, nil
	}

	if requested == "~" || strings.HasPrefix(requested, "~"+string(filepath.Separator)) {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolving home directory: %w", err)
		}
		if requested == "~" {
			requested = home
		} else {
			requested = filepath.Join(home, strings.TrimPrefix(requested, "~"+string(filepath.Separator)))
		}
	}

	if !filepath.IsAbs(requested) {
		abs, err := filepath.Abs(requested)
		if err != nil {
			return "", fmt.Errorf("resolving project directory: %w", err)
		}
		requested = abs
	}

	info, err := os.Stat(requested)
	if err != nil {
		return "", fmt.Errorf("project directory %q is not accessible: %w", requested, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("project directory %q is not a directory", requested)
	}
	return requested, nil
}

func claudeLaunchSettingsJSON(serverURL string) string {
	payload := map[string]interface{}{
		"env": map[string]string{
			"ANTHROPIC_BASE_URL":   serverURL,
			"ANTHROPIC_AUTH_TOKEN": "csghub-lite",
			"ANTHROPIC_API_KEY":    "csghub-lite",
			"CLAUDE_API_BASE_URL":  serverURL,
			"CLAUDE_API_KEY":       "csghub-lite",
		},
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return `{"env":{}}`
	}
	return string(data)
}

func writeOpenCodeWebLaunchConfig(serverURL, defaultModel string, modelIDs []string) (string, error) {
	dir, err := openCodeLaunchDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("creating OpenCode web launch config dir: %w", err)
	}

	modelMap := make(map[string]interface{}, len(modelIDs))
	for _, modelID := range modelIDs {
		modelMap[modelID] = map[string]interface{}{
			"name": modelID,
		}
	}

	payload := map[string]interface{}{
		"$schema":           "https://opencode.ai/config.json",
		"enabled_providers": []string{openCodeWebProviderID},
		"provider": map[string]interface{}{
			openCodeWebProviderID: map[string]interface{}{
				"npm":  "@ai-sdk/openai-compatible",
				"name": "CSGHub Lite",
				"options": map[string]interface{}{
					"baseURL": strings.TrimRight(serverURL, "/") + "/v1",
				},
				"models": modelMap,
			},
		},
		"model":       openCodeWebProviderID + "/" + defaultModel,
		"small_model": openCodeWebProviderID + "/" + defaultModel,
	}

	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return "", fmt.Errorf("encoding OpenCode web launch config: %w", err)
	}

	path := filepath.Join(dir, "opencode-web.json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", fmt.Errorf("writing OpenCode web launch config: %w", err)
	}
	return path, nil
}

func openCodeLaunchDir() (string, error) {
	appHome, err := config.AppHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(appHome, "apps", "launch"), nil
}

func resolveAIAppLaunchBinary(candidates []string) (string, error) {
	pathEnv := prependMissingPathEntries(os.Getenv("PATH"), commonAIAppBinDirs())
	_ = os.Setenv("PATH", pathEnv)

	for _, name := range candidates {
		if path, err := exec.LookPath(name); err == nil {
			return path, nil
		}
	}

	for _, dir := range commonAIAppBinDirs() {
		for _, name := range candidates {
			if path, ok := lookupAIAppBinaryInDir(dir, name); ok {
				return path, nil
			}
		}
	}

	return "", fmt.Errorf("command not found")
}

func prependMissingPathEntries(current string, extras []string) string {
	items := strings.Split(current, string(os.PathListSeparator))
	seen := make(map[string]struct{}, len(items)+len(extras))
	filtered := make([]string, 0, len(items)+len(extras))
	for _, item := range items {
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		filtered = append(filtered, item)
	}
	for _, item := range extras {
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		filtered = append([]string{item}, filtered...)
		seen[item] = struct{}{}
	}
	return strings.Join(filtered, string(os.PathListSeparator))
}

func commonAIAppBinDirs() []string {
	home, _ := os.UserHomeDir()
	dirs := []string{"/opt/homebrew/bin", "/usr/local/bin"}
	if home != "" {
		dirs = append([]string{
			filepath.Join(home, "bin"),
			filepath.Join(home, ".local", "bin"),
		}, dirs...)
	}
	if runtime.GOOS == "windows" {
		if appData := os.Getenv("APPDATA"); appData != "" {
			dirs = append([]string{filepath.Join(appData, "npm")}, dirs...)
		}
	}
	return uniqueNonEmptyStrings(dirs)
}

func uniqueNonEmptyStrings(items []string) []string {
	seen := make(map[string]struct{}, len(items))
	unique := make([]string, 0, len(items))
	for _, item := range items {
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		unique = append(unique, item)
	}
	return unique
}

func lookupAIAppBinaryInDir(dir, name string) (string, bool) {
	exts := []string{""}
	if runtime.GOOS == "windows" {
		exts = []string{"", ".exe", ".cmd", ".bat", ".ps1"}
	}
	for _, ext := range exts {
		path := filepath.Join(dir, name+ext)
		info, err := os.Stat(path)
		if err == nil && !info.IsDir() {
			return path, true
		}
	}
	return "", false
}

func newAIAppShellSessionID() string {
	buf := make([]byte, 12)
	if _, err := crand.Read(buf); err != nil {
		return fmt.Sprintf("shell-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(buf)
}

func (s *Server) handleAppShellWS(w http.ResponseWriter, r *http.Request) {
	sessionID := strings.TrimSpace(r.PathValue("id"))
	if sessionID == "" {
		writeError(w, http.StatusBadRequest, "session id is required")
		return
	}

	session, ok := s.appShells.Get(sessionID)
	if !ok {
		writeError(w, http.StatusNotFound, "shell session not found")
		return
	}

	conn, err := aiAppShellUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	attach := session.Attach()
	defer session.Detach(attach.events)

	if err := conn.WriteJSON(attach.ready); err != nil {
		return
	}
	if len(attach.replay) > 0 {
		if err := conn.WriteMessage(websocket.BinaryMessage, attach.replay); err != nil {
			return
		}
	}
	if attach.exitMsg != nil {
		_ = conn.WriteJSON(attach.exitMsg)
		return
	}

	writerDone := make(chan struct{})
	go func() {
		defer close(writerDone)
		defer conn.Close()
		for event := range attach.events {
			if len(event.output) > 0 {
				if err := conn.WriteMessage(websocket.BinaryMessage, event.output); err != nil {
					return
				}
			}
			if event.exit != nil {
				_ = conn.WriteJSON(event.exit)
				return
			}
		}
	}()

	for {
		select {
		case <-writerDone:
			return
		default:
		}

		messageType, payload, err := conn.ReadMessage()
		if err != nil {
			return
		}

		switch messageType {
		case websocket.BinaryMessage:
			if err := session.WriteInput(payload); err != nil {
				return
			}
		case websocket.TextMessage:
			var message aiAppShellClientMessage
			if err := json.Unmarshal(payload, &message); err != nil {
				continue
			}
			if message.Type == "resize" {
				_ = session.Resize(message.Cols, message.Rows)
			}
		}
	}
}

func (s *Server) handleAppShellClose(w http.ResponseWriter, r *http.Request) {
	sessionID := strings.TrimSpace(r.PathValue("id"))
	if sessionID == "" {
		writeError(w, http.StatusBadRequest, "session id is required")
		return
	}
	if !s.appShells.Close(sessionID) {
		writeError(w, http.StatusNotFound, "shell session not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
