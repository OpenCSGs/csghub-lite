package server

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/opencsgs/csghub-lite/internal/config"
)

const (
	csgclawDefaultAddr         = "127.0.0.1:18080"
	csgclawOnboardTimeout      = 2 * time.Minute
	csgclawServeWait           = 12 * time.Second
	csgclawLogName             = "csgclaw.log"
	csgclawDefaultProviderName = "default"
)

func (s *Server) openCSGClawURL(ctx context.Context, modelID string) (string, error) {
	binary, err := resolveAIAppLaunchBinary([]string{"csgclaw"})
	if err != nil {
		return "", fmt.Errorf("CSGClaw is installed, but its launch command was not found on PATH")
	}

	requestedModel := strings.TrimSpace(modelID)
	resolvedModel, modelIDs, err := s.resolveCSGClawLaunchModels(ctx, requestedModel)
	if err != nil {
		return "", err
	}
	if requestedModel != "" {
		s.savePreferredAIAppModel("csgclaw", resolvedModel)
	}

	if err := s.onboardCSGClaw(ctx, binary, resolvedModel, modelIDs); err != nil {
		return "", err
	}

	// Always restart to pick up model/config changes (like openclaw --force).
	stopCSGClaw()
	if err := s.startCSGClawServe(binary); err != nil {
		return "", err
	}
	if err := waitForCSGClaw(csgclawServeWait); err != nil {
		return "", err
	}

	return "http://" + csgclawDefaultAddr + "/", nil
}

func (s *Server) resolveCSGClawLaunchModels(ctx context.Context, requestedModel string) (string, []string, error) {
	requestedModel = strings.TrimSpace(requestedModel)
	if requestedModel != "" {
		return s.resolveAIAppLaunchModels(ctx, requestedModel)
	}

	preferredModel := s.preferredAIAppModel("csgclaw")
	if preferredModel != "" {
		modelID, modelIDs, err := s.resolveAIAppLaunchModels(ctx, preferredModel)
		if err == nil {
			return modelID, modelIDs, nil
		}
		if strings.Contains(err.Error(), "is not available for AI Apps") {
			s.clearPreferredAIAppModel("csgclaw")
		} else {
			return "", nil, err
		}
	}

	return s.resolveAIAppLaunchModels(ctx, "")
}

func (s *Server) onboardCSGClaw(ctx context.Context, binary, modelID string, modelIDs []string) error {
	listenAddr := ""
	if s != nil && s.cfg != nil {
		listenAddr = s.cfg.ListenAddr
	}
	serverURL := csgclawReachableBaseURL(listenAddr, csgclawInterfaceAddrs())
	token := strings.TrimSpace(s.cfg.Token)
	apiKey := token
	if apiKey == "" {
		apiKey = "csghub-lite"
	}
	modelBaseURL := strings.TrimRight(serverURL, "/") + "/v1"

	onboardCtx, cancel := context.WithTimeout(ctx, csgclawOnboardTimeout)
	defer cancel()

	args := []string{
		"onboard",
	}
	if csgclawConfigNeedsManagerRecreate(modelBaseURL, apiKey, modelID) {
		args = append(args, "--force-recreate-manager")
	}
	models := csgclawOrderedModels(modelID, modelIDs)
	args = append(args,
		"--base-url", modelBaseURL,
		"--api-key", apiKey,
		"--models", strings.Join(models, ","),
	)

	cmd := exec.CommandContext(onboardCtx, binary, args...)
	output, err := cmd.CombinedOutput()
	if onboardCtx.Err() == context.DeadlineExceeded {
		return fmt.Errorf("configuring CSGClaw timed out after %s", csgclawOnboardTimeout)
	}
	if err != nil {
		msg := strings.TrimSpace(string(output))
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("configuring CSGClaw: %s", msg)
	}

	return nil
}

func (s *Server) startCSGClawServe(binary string) error {
	appHome, err := config.AppHome()
	if err != nil {
		return err
	}
	logDir := filepath.Join(appHome, "apps", "logs")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return fmt.Errorf("creating CSGClaw log dir: %w", err)
	}
	logPath := filepath.Join(logDir, csgclawLogName)
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("opening CSGClaw log: %w", err)
	}

	cmd := exec.Command(binary, "serve")
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if err := cmd.Start(); err != nil {
		_ = logFile.Close()
		return fmt.Errorf("starting CSGClaw serve: %w", err)
	}
	_ = logFile.Close()
	_ = cmd.Process.Release()
	log.Printf("started csgclaw serve (pid %d)", cmd.Process.Pid)
	return nil
}

// stopCSGClaw terminates any running csgclaw serve process so a fresh
// instance can be started with updated configuration.
func stopCSGClaw() {
	if !csgclawReachable() {
		return
	}
	_ = exec.Command("pkill", "-f", "csgclaw serve").Run()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if !csgclawReachable() {
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
}

func csgclawReachable() bool {
	conn, err := net.DialTimeout("tcp", csgclawDefaultAddr, 750*time.Millisecond)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func waitForCSGClaw(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", csgclawDefaultAddr, 750*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return nil
		}
		time.Sleep(300 * time.Millisecond)
	}
	return fmt.Errorf("CSGClaw server did not become ready in time")
}

func csgclawReachableBaseURL(listenAddr string, addrs []net.Addr) string {
	host, port := csgclawListenHostPort(listenAddr)
	if csgclawNeedsReachableHost(host) {
		if reachableHost := csgclawReachableHost(addrs); reachableHost != "" {
			host = reachableHost
		} else {
			host = "127.0.0.1"
		}
	}
	return "http://" + net.JoinHostPort(host, port)
}

func csgclawListenHostPort(listenAddr string) (host, port string) {
	addr := strings.TrimSpace(listenAddr)
	if addr == "" {
		addr = config.DefaultListenAddr
	}
	if strings.HasPrefix(addr, ":") {
		return "", strings.TrimPrefix(addr, ":")
	}
	if host, port, err := net.SplitHostPort(addr); err == nil {
		return strings.Trim(host, "[]"), port
	}
	if strings.Count(addr, ":") == 1 {
		parts := strings.SplitN(addr, ":", 2)
		return parts[0], parts[1]
	}
	return "127.0.0.1", strings.TrimPrefix(config.DefaultListenAddr, ":")
}

func csgclawNeedsReachableHost(host string) bool {
	host = strings.TrimSpace(strings.Trim(host, "[]"))
	if host == "" || strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	return ip.IsLoopback() || ip.IsUnspecified()
}

func csgclawReachableHost(addrs []net.Addr) string {
	privateHosts := make([]string, 0, len(addrs))
	publicHosts := make([]string, 0, len(addrs))
	seen := make(map[string]struct{}, len(addrs))
	for _, addr := range addrs {
		var ip net.IP
		switch v := addr.(type) {
		case *net.IPNet:
			ip = v.IP
		case *net.IPAddr:
			ip = v.IP
		}
		if ip == nil || ip.IsLoopback() {
			continue
		}
		ip = ip.To4()
		if ip == nil {
			continue
		}
		host := ip.String()
		if _, ok := seen[host]; ok {
			continue
		}
		seen[host] = struct{}{}
		if ip.IsPrivate() {
			privateHosts = append(privateHosts, host)
			continue
		}
		if ip.IsGlobalUnicast() {
			publicHosts = append(publicHosts, host)
		}
	}
	if len(privateHosts) > 0 {
		return privateHosts[0]
	}
	if len(publicHosts) > 0 {
		return publicHosts[0]
	}
	return ""
}

func csgclawInterfaceAddrs() []net.Addr {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil
	}
	return addrs
}

func csgclawConfigNeedsManagerRecreate(baseURL, apiKey, modelID string) bool {
	cfg, err := readCSGClawModelConfig()
	if err != nil {
		return true
	}
	providerName := cfg.EffectiveProviderName()
	provider, ok := cfg.Providers[providerName]
	if !ok {
		return true
	}
	wantSelector := csgclawModelSelector(providerName, modelID)
	return strings.TrimSpace(cfg.DefaultSelector) != wantSelector ||
		strings.TrimRight(provider.BaseURL, "/") != strings.TrimRight(baseURL, "/") ||
		strings.TrimSpace(provider.APIKey) != strings.TrimSpace(apiKey) ||
		!csgclawContainsModel(provider.Models, modelID)
}

type csgclawModelConfig struct {
	DefaultSelector string
	Providers       map[string]csgclawModelProviderConfig
}

type csgclawModelProviderConfig struct {
	BaseURL string
	APIKey  string
	Models  []string
}

func (c csgclawModelConfig) EffectiveProviderName() string {
	selector := strings.TrimSpace(c.DefaultSelector)
	if providerName, _, ok := strings.Cut(selector, "."); ok {
		providerName = strings.TrimSpace(providerName)
		if providerName != "" {
			return providerName
		}
	}
	if len(c.Providers) == 1 {
		for name := range c.Providers {
			name = strings.TrimSpace(name)
			if name != "" {
				return name
			}
		}
	}
	if _, ok := c.Providers[csgclawDefaultProviderName]; ok {
		return csgclawDefaultProviderName
	}
	return ""
}

func readCSGClawModelConfig() (csgclawModelConfig, error) {
	path, err := csgclawConfigPath()
	if err != nil {
		return csgclawModelConfig{}, err
	}
	file, err := os.Open(path)
	if err != nil {
		return csgclawModelConfig{}, err
	}
	defer file.Close()

	cfg := csgclawModelConfig{
		Providers: make(map[string]csgclawModelProviderConfig),
	}
	section := ""
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.Trim(line, "[]")
			continue
		}
		key, value, ok := parseCSGClawConfigKV(line)
		if !ok {
			continue
		}
		switch {
		case section == "models":
			if key == "default" {
				cfg.DefaultSelector = value
			}
		case strings.HasPrefix(section, "models.providers."):
			providerName := strings.TrimSpace(strings.TrimPrefix(section, "models.providers."))
			if providerName == "" {
				continue
			}
			provider := cfg.Providers[providerName]
			switch key {
			case "base_url":
				provider.BaseURL = value
			case "api_key":
				provider.APIKey = value
			case "models":
				models, err := parseCSGClawConfigStringArray(value)
				if err != nil {
					return csgclawModelConfig{}, err
				}
				provider.Models = models
			}
			cfg.Providers[providerName] = provider
		}
	}
	if err := scanner.Err(); err != nil {
		return csgclawModelConfig{}, err
	}
	providerName := cfg.EffectiveProviderName()
	provider, ok := cfg.Providers[providerName]
	if providerName == "" || !ok || strings.TrimSpace(cfg.DefaultSelector) == "" || strings.TrimSpace(provider.BaseURL) == "" || len(provider.Models) == 0 {
		return csgclawModelConfig{}, fmt.Errorf("csgclaw models config is incomplete")
	}
	return cfg, nil
}

func csgclawConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".csgclaw", "config.toml"), nil
}

func parseCSGClawConfigKV(line string) (key, value string, ok bool) {
	key, value, ok = strings.Cut(line, "=")
	if !ok {
		return "", "", false
	}
	key = strings.TrimSpace(key)
	value = strings.TrimSpace(value)
	if len(value) >= 2 && strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"") {
		if unquoted, err := strconv.Unquote(value); err == nil {
			value = unquoted
		} else {
			value = strings.Trim(value, "\"")
		}
	}
	return key, value, true
}

func parseCSGClawConfigStringArray(value string) ([]string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	if !strings.HasPrefix(value, "[") || !strings.HasSuffix(value, "]") {
		return nil, fmt.Errorf("invalid csgclaw array value %q", value)
	}
	inner := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(value, "["), "]"))
	if inner == "" {
		return nil, nil
	}
	items := strings.Split(inner, ",")
	models := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if len(item) >= 2 && strings.HasPrefix(item, "\"") && strings.HasSuffix(item, "\"") {
			unquoted, err := strconv.Unquote(item)
			if err != nil {
				return nil, err
			}
			item = unquoted
		}
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		models = append(models, item)
	}
	return models, nil
}

func csgclawModelSelector(providerName, modelID string) string {
	providerName = strings.TrimSpace(providerName)
	modelID = strings.TrimSpace(modelID)
	if providerName == "" || modelID == "" {
		return ""
	}
	return providerName + "." + modelID
}

func csgclawContainsModel(models []string, want string) bool {
	want = strings.TrimSpace(want)
	for _, model := range models {
		if strings.TrimSpace(model) == want {
			return true
		}
	}
	return false
}

func csgclawOrderedModels(selected string, modelIDs []string) []string {
	selected = strings.TrimSpace(selected)
	ordered := make([]string, 0, len(modelIDs)+1)
	seen := make(map[string]struct{}, len(modelIDs)+1)
	appendModel := func(modelID string) {
		modelID = strings.TrimSpace(modelID)
		if modelID == "" {
			return
		}
		if _, ok := seen[modelID]; ok {
			return
		}
		seen[modelID] = struct{}{}
		ordered = append(ordered, modelID)
	}

	appendModel(selected)
	for _, modelID := range modelIDs {
		appendModel(modelID)
	}
	return ordered
}
