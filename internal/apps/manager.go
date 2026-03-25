package apps

import (
	"bufio"
	"bytes"
	"context"
	"embed"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/opencsgs/csghub-lite/internal/config"
	"github.com/opencsgs/csghub-lite/pkg/api"
)

const (
	progressModePercent       = "percent"
	progressModeIndeterminate = "indeterminate"
	mirrorBaseURL             = "https://git-devops.opencsg.com/opensource/apps/-/raw/main"
	installTimeout            = 20 * time.Minute
)

//go:embed scripts/*
var embeddedScripts embed.FS

type scriptSource struct {
	mirrorURL    string
	embeddedPath string
	args         []string
}

type appSpec struct {
	id             string
	binaryName     string
	installMode    string
	progressMode   string
	supported      bool
	disabledReason string
	versionArgs    []string
	unix           *scriptSource
	windows        *scriptSource
	uninstallUnix  *scriptSource
	uninstallWin   *scriptSource
}

type appState struct {
	info    api.AIAppInfo
	logBuf  *LogBuffer
	cancel  context.CancelFunc
	running bool
}

type Manager struct {
	cfg        *config.Config
	httpClient *http.Client

	mu     sync.RWMutex
	specs  []appSpec
	states map[string]*appState
}

func NewManager(cfg *config.Config) *Manager {
	m := &Manager{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: 20 * time.Second,
		},
		specs:  appSpecs(),
		states: make(map[string]*appState),
	}

	for _, spec := range m.specs {
		logPath := m.logPath(spec.id)
		status := "idle"
		phase := "ready"
		if !spec.supported {
			status = "disabled"
			phase = "docker_disabled"
		}
		m.states[spec.id] = &appState{
			info: api.AIAppInfo{
				ID:             spec.id,
				Supported:      spec.supported,
				Disabled:       !spec.supported,
				Status:         status,
				Phase:          phase,
				ProgressMode:   spec.progressMode,
				LogPath:        logPath,
				DisabledReason: spec.disabledReason,
				UpdatedAt:      time.Now(),
			},
			logBuf: NewLogBuffer(500),
		}
	}

	_ = m.RefreshAll(context.Background())
	return m
}

func appSpecs() []appSpec {
	return []appSpec{
		{
			id:           "claude-code",
			binaryName:   "claude",
			installMode:  "script",
			progressMode: progressModeIndeterminate,
			supported:    true,
			versionArgs:  []string{"--version"},
			unix: &scriptSource{
				mirrorURL:    mirrorBaseURL + "/claude-code/install.sh",
				embeddedPath: "scripts/claude-code-install.sh",
				args:         []string{"latest"},
			},
			windows: &scriptSource{
				mirrorURL:    mirrorBaseURL + "/claude-code/install.ps1",
				embeddedPath: "scripts/claude-code-install.ps1",
				args:         []string{"-Target", "latest"},
			},
			uninstallUnix: &scriptSource{
				embeddedPath: "scripts/claude-code-uninstall.sh",
			},
			uninstallWin: &scriptSource{
				embeddedPath: "scripts/claude-code-uninstall.ps1",
			},
		},
		{
			id:           "open-code",
			binaryName:   "opencode",
			installMode:  "npm",
			progressMode: progressModePercent,
			supported:    true,
			versionArgs:  []string{"--version"},
			unix: &scriptSource{
				mirrorURL:    mirrorBaseURL + "/open-code/install.sh",
				embeddedPath: "scripts/open-code-install.sh",
			},
			windows: &scriptSource{
				mirrorURL:    mirrorBaseURL + "/open-code/install.ps1",
				embeddedPath: "scripts/open-code-install.ps1",
			},
			uninstallUnix: &scriptSource{
				embeddedPath: "scripts/open-code-uninstall.sh",
			},
			uninstallWin: &scriptSource{
				embeddedPath: "scripts/open-code-uninstall.ps1",
			},
		},
		{
			id:           "openclaw",
			binaryName:   "openclaw",
			installMode:  "script",
			progressMode: progressModePercent,
			supported:    true,
			versionArgs:  []string{"--version"},
			unix: &scriptSource{
				mirrorURL:    mirrorBaseURL + "/openclaw/install.sh",
				embeddedPath: "scripts/openclaw-install.sh",
			},
			windows: &scriptSource{
				mirrorURL:    mirrorBaseURL + "/openclaw/install.ps1",
				embeddedPath: "scripts/openclaw-install.ps1",
			},
			uninstallUnix: &scriptSource{
				embeddedPath: "scripts/openclaw-uninstall.sh",
			},
			uninstallWin: &scriptSource{
				embeddedPath: "scripts/openclaw-uninstall.ps1",
			},
		},
		{
			id:           "codex",
			binaryName:   "codex",
			installMode:  "npm",
			progressMode: progressModePercent,
			supported:    true,
			versionArgs:  []string{"--version"},
			unix: &scriptSource{
				mirrorURL:    mirrorBaseURL + "/codex/install.sh",
				embeddedPath: "scripts/codex-install.sh",
			},
			windows: &scriptSource{
				mirrorURL:    mirrorBaseURL + "/codex/install.ps1",
				embeddedPath: "scripts/codex-install.ps1",
			},
			uninstallUnix: &scriptSource{
				embeddedPath: "scripts/codex-uninstall.sh",
			},
			uninstallWin: &scriptSource{
				embeddedPath: "scripts/codex-uninstall.ps1",
			},
		},
		{
			id:             "dify",
			installMode:    "docker",
			progressMode:   progressModeIndeterminate,
			supported:      false,
			disabledReason: "docker_disabled",
		},
		{
			id:             "anythingllm",
			installMode:    "docker",
			progressMode:   progressModeIndeterminate,
			supported:      false,
			disabledReason: "docker_disabled",
		},
	}
}

func (m *Manager) List(ctx context.Context) ([]api.AIAppInfo, error) {
	if err := m.RefreshAll(ctx); err != nil {
		return nil, err
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	items := make([]api.AIAppInfo, 0, len(m.specs))
	for _, spec := range m.specs {
		st := m.states[spec.id]
		items = append(items, cloneInfo(st.info))
	}
	return items, nil
}

func (m *Manager) Get(ctx context.Context, appID string) (api.AIAppInfo, error) {
	if err := m.RefreshAll(ctx); err != nil {
		return api.AIAppInfo{}, err
	}

	m.mu.RLock()
	defer m.mu.RUnlock()
	st, ok := m.states[appID]
	if !ok {
		return api.AIAppInfo{}, fmt.Errorf("unknown app %q", appID)
	}
	return cloneInfo(st.info), nil
}

func (m *Manager) RefreshAll(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, spec := range m.specs {
		st := m.states[spec.id]
		if st == nil || st.running || !spec.supported {
			continue
		}

		installPath, version, installed := detectInstalled(ctx, spec)
		st.info.Installed = installed
		st.info.InstallPath = installPath
		st.info.Version = version
		st.info.ProgressMode = spec.progressMode
		st.info.UpdatedAt = time.Now()

		if installed {
			st.info.Status = "installed"
			if st.info.Phase == "uninstall_failed" && st.info.LastError != "" {
				st.info.Phase = "uninstall_failed"
			} else {
				st.info.Phase = "installed"
				st.info.LastError = ""
			}
			st.info.Progress = 100
			continue
		}

		if st.info.Status == "failed" {
			if st.info.Phase == "" {
				st.info.Phase = "failed"
			}
			st.info.Progress = 0
			continue
		}

		st.info.Status = "idle"
		st.info.Phase = "ready"
		st.info.Progress = 0
	}
	return nil
}

func (m *Manager) Install(appID string) (api.AIAppInfo, error) {
	return m.startAction(appID, "install")
}

func (m *Manager) Uninstall(appID string) (api.AIAppInfo, error) {
	return m.startAction(appID, "uninstall")
}

func (m *Manager) startAction(appID, action string) (api.AIAppInfo, error) {
	m.mu.Lock()
	spec, st, err := m.specStateLocked(appID)
	if err != nil {
		m.mu.Unlock()
		return api.AIAppInfo{}, err
	}
	if !spec.supported {
		info := cloneInfo(st.info)
		m.mu.Unlock()
		return info, errors.New("app is disabled")
	}
	if st.running {
		info := cloneInfo(st.info)
		m.mu.Unlock()
		return info, nil
	}
	if action == "uninstall" && !st.info.Installed {
		info := cloneInfo(st.info)
		m.mu.Unlock()
		return info, nil
	}

	st.logBuf.Reset()
	if err := os.MkdirAll(filepath.Dir(st.info.LogPath), 0o755); err != nil {
		m.mu.Unlock()
		return api.AIAppInfo{}, err
	}
	if err := os.WriteFile(st.info.LogPath, nil, 0o644); err != nil {
		m.mu.Unlock()
		return api.AIAppInfo{}, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), installTimeout)
	st.cancel = cancel
	st.running = true
	st.info.Status = actionStatus(action)
	st.info.Phase = "starting"
	st.info.Progress = 0
	st.info.LastError = ""
	st.info.UpdatedAt = time.Now()
	info := cloneInfo(st.info)
	m.mu.Unlock()

	go m.runAction(ctx, spec, action)
	return info, nil
}

func (m *Manager) RecentLogs(appID string, n int) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	st, ok := m.states[appID]
	if !ok {
		return nil, fmt.Errorf("unknown app %q", appID)
	}
	return st.logBuf.Recent(n), nil
}

func (m *Manager) SubscribeLogs(appID string) (chan string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	st, ok := m.states[appID]
	if !ok {
		return nil, fmt.Errorf("unknown app %q", appID)
	}
	return st.logBuf.Subscribe(), nil
}

func (m *Manager) UnsubscribeLogs(appID string, ch chan string) {
	m.mu.RLock()
	st := m.states[appID]
	m.mu.RUnlock()
	if st != nil {
		st.logBuf.Unsubscribe(ch)
	}
}

func actionStatus(action string) string {
	if action == "uninstall" {
		return "uninstalling"
	}
	return "installing"
}

func actionRunnerName(action string) string {
	if action == "uninstall" {
		return "uninstaller"
	}
	return "installer"
}

func (m *Manager) runAction(ctx context.Context, spec appSpec, action string) {
	logFile, err := os.OpenFile(m.logPath(spec.id), os.O_CREATE|os.O_WRONLY|os.O_TRUNC|os.O_APPEND, 0o644)
	if err != nil {
		m.failAction(spec, action, fmt.Sprintf("open log file: %v", err))
		return
	}
	defer logFile.Close()

	runnerName := actionRunnerName(action)
	m.appendLog(spec.id, logFile, fmt.Sprintf("INFO: preparing %s", runnerName))
	source, err := m.currentScriptSource(spec, action)
	if err != nil {
		m.failAction(spec, action, err.Error())
		return
	}

	content, resolvedFrom, err := m.resolveScript(spec.id, source)
	if err != nil {
		m.failAction(spec, action, err.Error())
		return
	}

	m.appendLog(spec.id, logFile, fmt.Sprintf("INFO: %s source %s", runnerName, resolvedFrom))
	m.updateProgress(spec.id, 5, "preflight")

	tmpPath, err := m.writeTempScript(spec.id, source, content)
	if err != nil {
		m.failAction(spec, action, err.Error())
		return
	}
	defer os.Remove(tmpPath)

	cmd, err := buildScriptCommand(tmpPath, source)
	if err != nil {
		m.failAction(spec, action, err.Error())
		return
	}
	cmd = exec.CommandContext(ctx, cmd.Path, cmd.Args[1:]...)
	cmd.Env = append(os.Environ(), cmd.Env...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		m.failAction(spec, action, err.Error())
		return
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		m.failAction(spec, action, err.Error())
		return
	}

	if err := cmd.Start(); err != nil {
		m.failAction(spec, action, err.Error())
		return
	}
	m.appendLog(spec.id, logFile, fmt.Sprintf("INFO: running %s", strings.Join(cmd.Args, " ")))

	var wg sync.WaitGroup
	wg.Add(2)
	go m.consumeOutput(&wg, spec.id, stdout, logFile)
	go m.consumeOutput(&wg, spec.id, stderr, logFile)
	wg.Wait()

	if err := cmd.Wait(); err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			m.failAction(spec, action, fmt.Sprintf("%s timed out after %s", runnerName, installTimeout))
			return
		}
		m.failAction(spec, action, err.Error())
		return
	}

	verifyPhase := "verifying"
	verifyErr := "installer completed but binary was not found on PATH"
	if action == "uninstall" {
		verifyPhase = "verifying_uninstall"
		verifyErr = "uninstaller completed but binary is still found on PATH"
	}
	m.updateProgress(spec.id, 95, verifyPhase)
	installPath, version, installed := detectInstalled(context.Background(), spec)
	if action == "uninstall" {
		if installed {
			m.failAction(spec, action, verifyErr)
			return
		}
		m.completeUninstall(spec)
		m.appendLog(spec.id, logFile, "INFO: uninstallation complete")
		return
	}

	if !installed {
		m.failAction(spec, action, verifyErr)
		return
	}

	m.completeInstall(spec, installPath, version)
	m.appendLog(spec.id, logFile, "INFO: installation complete")
}

func (m *Manager) consumeOutput(wg *sync.WaitGroup, appID string, r io.Reader, logFile *os.File) {
	defer wg.Done()
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if handled := m.handleControlLine(appID, line); handled {
			continue
		}
		m.appendLog(appID, logFile, line)
	}
	if err := scanner.Err(); err != nil {
		m.appendLog(appID, logFile, fmt.Sprintf("WARN: stream read error: %v", err))
	}
}

func (m *Manager) handleControlLine(appID, line string) bool {
	if !strings.HasPrefix(line, "CSGHUB_PROGRESS|") {
		return false
	}
	parts := strings.SplitN(line, "|", 3)
	if len(parts) != 3 {
		return true
	}
	value, err := strconv.Atoi(parts[1])
	if err != nil {
		return true
	}
	phase := parts[2]
	m.updateProgress(appID, value, phase)
	return true
}

func (m *Manager) updateProgress(appID string, progress int, phase string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	st := m.states[appID]
	if st == nil {
		return
	}
	if st.info.Status != "installing" && st.info.Status != "uninstalling" {
		st.info.Status = "installing"
	}
	st.info.Phase = phase
	if st.info.ProgressMode == progressModePercent {
		st.info.Progress = progress
	}
	st.info.UpdatedAt = time.Now()
}

func (m *Manager) completeInstall(spec appSpec, installPath, version string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	st := m.states[spec.id]
	if st == nil {
		return
	}
	st.running = false
	st.cancel = nil
	st.info.Status = "installed"
	st.info.Phase = "installed"
	st.info.Progress = 100
	st.info.Installed = true
	st.info.InstallPath = installPath
	st.info.Version = version
	st.info.LastError = ""
	st.info.UpdatedAt = time.Now()
}

func (m *Manager) completeUninstall(spec appSpec) {
	m.mu.Lock()
	defer m.mu.Unlock()
	st := m.states[spec.id]
	if st == nil {
		return
	}
	st.running = false
	st.cancel = nil
	st.info.Status = "idle"
	st.info.Phase = "ready"
	st.info.Progress = 0
	st.info.Installed = false
	st.info.InstallPath = ""
	st.info.Version = ""
	st.info.LastError = ""
	st.info.UpdatedAt = time.Now()
}

func (m *Manager) failAction(spec appSpec, action, errMsg string) {
	installPath, version, installed := detectInstalled(context.Background(), spec)
	m.mu.Lock()
	defer m.mu.Unlock()
	st := m.states[spec.id]
	if st == nil {
		return
	}
	st.running = false
	st.cancel = nil
	if action == "uninstall" {
		if installed {
			st.info.Status = "installed"
			st.info.Phase = "uninstall_failed"
			st.info.Progress = 100
			st.info.Installed = true
			st.info.InstallPath = installPath
			st.info.Version = version
		} else {
			st.info.Status = "failed"
			st.info.Phase = "uninstall_failed"
			st.info.Progress = 0
			st.info.Installed = false
			st.info.InstallPath = ""
			st.info.Version = ""
		}
	} else if installed {
		st.info.Status = "installed"
		st.info.Phase = "installed"
		st.info.Progress = 100
		st.info.Installed = true
		st.info.InstallPath = installPath
		st.info.Version = version
	} else {
		st.info.Status = "failed"
		st.info.Phase = "failed"
		st.info.Progress = 0
		st.info.Installed = false
		st.info.InstallPath = ""
		st.info.Version = ""
	}
	st.info.LastError = errMsg
	st.info.UpdatedAt = time.Now()
}

func (m *Manager) appendLog(appID string, logFile *os.File, line string) {
	m.mu.RLock()
	st := m.states[appID]
	m.mu.RUnlock()
	if st == nil {
		return
	}
	formatted := fmt.Sprintf("%s %s", time.Now().Format("2006-01-02 15:04:05"), trimLine(line))
	st.logBuf.Append(formatted)
	if logFile != nil {
		_, _ = logFile.WriteString(formatted + "\n")
	}
}

func (m *Manager) resolveScript(appID string, source *scriptSource) ([]byte, string, error) {
	if source == nil {
		return nil, "", fmt.Errorf("no script configured for %s on %s", appID, runtime.GOOS)
	}
	if source.mirrorURL != "" {
		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, source.mirrorURL, nil)
		if err == nil {
			resp, err := m.httpClient.Do(req)
			if err == nil {
				defer resp.Body.Close()
				if resp.StatusCode >= 200 && resp.StatusCode < 300 {
					data, err := io.ReadAll(resp.Body)
					if err == nil {
						return data, source.mirrorURL, nil
					}
				}
			}
		}
	}

	data, err := embeddedScripts.ReadFile(source.embeddedPath)
	if err != nil {
		return nil, "", fmt.Errorf("read embedded script: %w", err)
	}
	return data, "embedded:" + source.embeddedPath, nil
}

func (m *Manager) writeTempScript(appID string, source *scriptSource, content []byte) (string, error) {
	ext := ".sh"
	if runtime.GOOS == "windows" {
		ext = ".ps1"
	}
	tmp, err := os.CreateTemp("", appID+"-*"+ext)
	if err != nil {
		return "", err
	}
	defer tmp.Close()
	if _, err := io.Copy(tmp, bytes.NewReader(content)); err != nil {
		return "", err
	}
	if runtime.GOOS != "windows" {
		if err := os.Chmod(tmp.Name(), 0o755); err != nil {
			return "", err
		}
	}
	return tmp.Name(), nil
}

func buildScriptCommand(scriptPath string, source *scriptSource) (*exec.Cmd, error) {
	if runtime.GOOS == "windows" {
		powershell, err := exec.LookPath("powershell")
		if err != nil {
			return nil, fmt.Errorf("powershell not found: %w", err)
		}
		args := []string{"-NoProfile", "-ExecutionPolicy", "Bypass", "-File", scriptPath}
		args = append(args, source.args...)
		return exec.Command(powershell, args...), nil
	}

	bash, err := exec.LookPath("bash")
	if err != nil {
		return nil, fmt.Errorf("bash not found: %w", err)
	}

	shellPath := os.Getenv("SHELL")
	if shellPath == "" {
		shellPath = bash
	}
	if _, err := os.Stat(shellPath); err != nil {
		shellPath = bash
	}

	args := []string{
		"-lc",
		`if [ -f "$HOME/.myshrc" ]; then . "$HOME/.myshrc" >/dev/null 2>&1 || true; fi; exec "$@"`,
		"csghub-app-installer",
		bash,
		scriptPath,
	}
	args = append(args, source.args...)
	return exec.Command(shellPath, args...), nil
}

func (m *Manager) currentScriptSource(spec appSpec, action string) (*scriptSource, error) {
	if runtime.GOOS == "windows" {
		if action == "uninstall" {
			if spec.uninstallWin == nil {
				return nil, fmt.Errorf("no Windows uninstaller configured for %s", spec.id)
			}
			return spec.uninstallWin, nil
		}
		if spec.windows == nil {
			return nil, fmt.Errorf("no Windows installer configured for %s", spec.id)
		}
		return spec.windows, nil
	}
	if action == "uninstall" {
		if spec.uninstallUnix == nil {
			return nil, fmt.Errorf("no Unix uninstaller configured for %s", spec.id)
		}
		return spec.uninstallUnix, nil
	}
	if spec.unix == nil {
		return nil, fmt.Errorf("no Unix installer configured for %s", spec.id)
	}
	return spec.unix, nil
}

func detectInstalled(ctx context.Context, spec appSpec) (string, string, bool) {
	if spec.binaryName == "" {
		return "", "", false
	}
	path, err := exec.LookPath(spec.binaryName)
	if err != nil {
		return "", "", false
	}
	version := path
	if len(spec.versionArgs) > 0 {
		cmdCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		out, err := exec.CommandContext(cmdCtx, path, spec.versionArgs...).CombinedOutput()
		if err == nil {
			version = strings.TrimSpace(string(out))
		}
	}
	return path, version, true
}

func (m *Manager) logPath(appID string) string {
	home, err := config.AppHome()
	if err != nil {
		return filepath.Join(os.TempDir(), appID+".log")
	}
	return filepath.Join(home, "apps", "logs", appID+".log")
}

func (m *Manager) specStateLocked(appID string) (appSpec, *appState, error) {
	for _, spec := range m.specs {
		if spec.id == appID {
			st, ok := m.states[appID]
			if !ok {
				return appSpec{}, nil, fmt.Errorf("unknown app %q", appID)
			}
			return spec, st, nil
		}
	}
	return appSpec{}, nil, fmt.Errorf("unknown app %q", appID)
}

func trimLine(line string) string {
	line = strings.TrimRight(line, "\r\n")
	if line == "" {
		return "(empty line)"
	}
	return line
}

func cloneInfo(info api.AIAppInfo) api.AIAppInfo {
	return info
}
