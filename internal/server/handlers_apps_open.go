package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	neturl "net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/opencsgs/csghub-lite/internal/config"
	"github.com/opencsgs/csghub-lite/internal/model"
)

const (
	openClawWebProfile      = "csghub-lite"
	openClawProviderID      = "csghub"
	openClawOpenTimeout     = 2 * time.Minute
	openClawGatewayWait     = 12 * time.Second
	openClawGatewayLogName  = "openclaw-gateway.log"
	openClawDashboardPrefix = "Dashboard URL:"
)

func (s *Server) openAIAppURL(ctx context.Context, appID, modelID, workDir string) (string, error) {
	info, err := s.appManager.Get(ctx, appID)
	if err != nil {
		return "", err
	}
	if info.Disabled || !info.Supported {
		return "", fmt.Errorf("%s is currently disabled in AI Apps", appID)
	}
	if !info.Installed {
		return "", fmt.Errorf("%s is not installed yet", appID)
	}

	switch appID {
	case "openclaw":
		return s.openClawChatURL(ctx, modelID)
	case "claude-code", "open-code", "codex":
		return s.openAIAppShellURL(ctx, appID, modelID, workDir)
	default:
		return "", fmt.Errorf("%s does not provide a direct chat entry yet", appID)
	}
}

func (s *Server) openClawChatURL(ctx context.Context, modelID string) (string, error) {
	binary, err := exec.LookPath("openclaw")
	if err != nil {
		return "", fmt.Errorf("OpenClaw is installed, but its launch command was not found on PATH")
	}

	s.refreshOpenClawModelCatalog(ctx)

	if err := s.ensureOpenClawProfile(ctx, binary, modelID); err != nil {
		return "", err
	}

	url, err := openClawDashboardURL(ctx, binary)
	if err != nil {
		return "", err
	}
	if dashboardReachable(url) {
		return openClawDirectChatURL(url, "main")
	}
	if err := s.startOpenClawGateway(binary); err != nil {
		return "", err
	}
	if err := waitForDashboard(url, openClawGatewayWait); err != nil {
		return "", err
	}
	return openClawDirectChatURL(url, "main")
}

func (s *Server) refreshOpenClawModelCatalog(ctx context.Context) {
	if s.cloud == nil || strings.TrimSpace(s.cfg.Token) == "" {
		return
	}

	s.cloud.InvalidateChatModels()
	if _, err := s.cloud.RefreshChatModels(ctx); err != nil {
		log.Printf("refreshing cloud models before opening OpenClaw failed: %v", err)
	}
}

func (s *Server) ensureOpenClawProfile(ctx context.Context, binary, requestedModelID string) error {
	modelID, _, err := s.resolveAIAppLaunchModels(ctx, requestedModelID)
	if err != nil {
		return err
	}
	serverURL := s.localBaseURL()

	ok, err := openClawProfileMatches(serverURL, modelID)
	if err == nil && ok {
		return nil
	}

	configureCtx, cancel := context.WithTimeout(ctx, openClawOpenTimeout)
	defer cancel()

	args := []string{
		"--profile", openClawWebProfile,
		"onboard",
		"--non-interactive",
		"--auth-choice", "custom-api-key",
		"--custom-provider-id", openClawProviderID,
		"--custom-compatibility", "openai",
		"--custom-base-url", openClawProviderBaseURL(serverURL),
		"--custom-model-id", modelID,
		"--custom-api-key", "csghub-lite",
		"--accept-risk",
		"--skip-channels",
		"--skip-search",
		"--skip-ui",
		"--skip-skills",
		"--skip-daemon",
		"--skip-health",
	}
	cmd := exec.CommandContext(configureCtx, binary, args...)
	output, err := cmd.CombinedOutput()
	if configureCtx.Err() == context.DeadlineExceeded {
		return fmt.Errorf("configuring OpenClaw timed out after %s", openClawOpenTimeout)
	}
	if err != nil {
		msg := strings.TrimSpace(string(output))
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("configuring OpenClaw: %s", msg)
	}
	return nil
}

func scoreOpenClawModel(m *model.LocalModel) int64 {
	name := strings.ToLower(m.FullName())
	score := m.Size / 1_000_000
	if strings.Contains(name, "coder") {
		score += 10_000_000
	}
	if strings.Contains(name, "code") {
		score += 5_000_000
	}
	if strings.Contains(name, "gpt-oss") {
		score += 6_000_000
	}
	if strings.Contains(name, "qwen") {
		score += 2_000_000
	}
	return score
}

func openClawProviderBaseURL(serverURL string) string {
	return strings.TrimRight(serverURL, "/") + "/v1"
}

func (s *Server) localBaseURL() string {
	addr := strings.TrimSpace(s.cfg.ListenAddr)
	if addr == "" {
		return "http://127.0.0.1" + config.DefaultListenAddr
	}
	if strings.HasPrefix(addr, ":") {
		return "http://127.0.0.1" + addr
	}
	if strings.HasPrefix(addr, "0.0.0.0:") {
		return "http://127.0.0.1:" + strings.TrimPrefix(addr, "0.0.0.0:")
	}
	if strings.HasPrefix(addr, "localhost:") || strings.HasPrefix(addr, "127.0.0.1:") {
		return "http://" + addr
	}
	if strings.Contains(addr, "://") {
		return addr
	}
	return "http://" + addr
}

func openClawDashboardURL(ctx context.Context, binary string) (string, error) {
	cmd := exec.CommandContext(ctx, binary, "--profile", openClawWebProfile, "dashboard", "--no-open")
	output, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(output))
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("fetching OpenClaw dashboard URL: %s", msg)
	}
	url, err := extractDashboardURL(output)
	if err != nil {
		return "", err
	}
	return url, nil
}

func extractDashboardURL(output []byte) (string, error) {
	for _, rawLine := range bytes.Split(output, []byte{'\n'}) {
		line := strings.TrimSpace(string(rawLine))
		if !strings.HasPrefix(line, openClawDashboardPrefix) {
			continue
		}
		url := strings.TrimSpace(strings.TrimPrefix(line, openClawDashboardPrefix))
		if url == "" {
			continue
		}
		if _, err := neturl.Parse(url); err == nil {
			return url, nil
		}
	}
	return "", fmt.Errorf("OpenClaw did not return a usable dashboard URL")
}

func openClawDirectChatURL(rawURL, session string) (string, error) {
	parsed, err := neturl.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("parsing OpenClaw dashboard URL: %w", err)
	}
	basePath := strings.TrimRight(parsed.Path, "/")
	parsed.Path = basePath + "/chat"
	query := parsed.Query()
	query.Set("session", session)
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func dashboardReachable(rawURL string) bool {
	hostPort, err := dashboardHostPort(rawURL)
	if err != nil {
		return false
	}
	conn, err := net.DialTimeout("tcp", hostPort, 750*time.Millisecond)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func waitForDashboard(rawURL string, timeout time.Duration) error {
	hostPort, err := dashboardHostPort(rawURL)
	if err != nil {
		return err
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", hostPort, 750*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return nil
		}
		time.Sleep(300 * time.Millisecond)
	}
	return fmt.Errorf("OpenClaw gateway did not become ready in time")
}

func dashboardHostPort(rawURL string) (string, error) {
	parsed, err := neturl.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("parsing OpenClaw dashboard URL: %w", err)
	}
	host := parsed.Hostname()
	port := parsed.Port()
	if host == "" || port == "" {
		return "", fmt.Errorf("OpenClaw dashboard URL is missing a host or port")
	}
	return net.JoinHostPort(host, port), nil
}

func (s *Server) startOpenClawGateway(binary string) error {
	appHome, err := config.AppHome()
	if err != nil {
		return err
	}
	logDir := filepath.Join(appHome, "apps", "logs")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return fmt.Errorf("creating OpenClaw gateway log dir: %w", err)
	}
	logPath := filepath.Join(logDir, openClawGatewayLogName)
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("opening OpenClaw gateway log: %w", err)
	}

	cmd := exec.Command(binary, "--profile", openClawWebProfile, "gateway", "run")
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if err := cmd.Start(); err != nil {
		_ = logFile.Close()
		return fmt.Errorf("starting OpenClaw gateway: %w", err)
	}
	_ = logFile.Close()
	_ = cmd.Process.Release()
	return nil
}

func openClawProfileMatches(serverURL, modelID string) (bool, error) {
	path, err := openClawProfileConfigPath()
	if err != nil {
		return false, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}

	var cfg struct {
		Models struct {
			Providers map[string]struct {
				BaseURL string `json:"baseUrl"`
			} `json:"providers"`
		} `json:"models"`
		Agents struct {
			Defaults struct {
				Model struct {
					Primary string `json:"primary"`
				} `json:"model"`
			} `json:"defaults"`
		} `json:"agents"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return false, err
	}

	provider, ok := cfg.Models.Providers[openClawProviderID]
	if !ok {
		return false, nil
	}
	wantModel := openClawProviderID + "/" + modelID
	return strings.TrimRight(provider.BaseURL, "/") == strings.TrimRight(openClawProviderBaseURL(serverURL), "/") &&
		cfg.Agents.Defaults.Model.Primary == wantModel, nil
}

func openClawProfileConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	base := ".openclaw-" + openClawWebProfile
	if openClawWebProfile == "" {
		base = ".openclaw"
	}
	return filepath.Join(home, base, "openclaw.json"), nil
}

func envWithOverrides(overrides map[string]string) []string {
	env := append([]string{}, os.Environ()...)
	for key, value := range overrides {
		prefix := key + "="
		replaced := false
		for i, item := range env {
			if strings.HasPrefix(item, prefix) {
				env[i] = prefix + value
				replaced = true
				break
			}
		}
		if !replaced {
			env = append(env, prefix+value)
		}
	}
	return env
}
