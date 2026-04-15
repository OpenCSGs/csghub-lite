package server

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/opencsgs/csghub-lite/internal/config"
)

const (
	csgclawDefaultAddr    = "127.0.0.1:18080"
	csgclawOnboardTimeout = 2 * time.Minute
	csgclawServeWait      = 12 * time.Second
	csgclawLogName        = "csgclaw.log"
)

func (s *Server) openCSGClawURL(ctx context.Context, modelID string) (string, error) {
	binary, err := resolveAIAppLaunchBinary([]string{"csgclaw"})
	if err != nil {
		return "", fmt.Errorf("CSGClaw is installed, but its launch command was not found on PATH")
	}

	resolvedModel, modelIDs, err := s.resolveAIAppLaunchModels(ctx, modelID)
	if err != nil {
		return "", err
	}

	if err := s.onboardCSGClaw(ctx, binary, resolvedModel, modelIDs); err != nil {
		return "", err
	}

	if !csgclawReachable() {
		if err := s.startCSGClawServe(binary); err != nil {
			return "", err
		}
		if err := waitForCSGClaw(csgclawServeWait); err != nil {
			return "", err
		}
	}

	return "http://" + csgclawDefaultAddr + "/", nil
}

func (s *Server) onboardCSGClaw(ctx context.Context, binary, modelID string, modelIDs []string) error {
	serverURL := s.localBaseURL()
	token := strings.TrimSpace(s.cfg.Token)
	apiKey := token
	if apiKey == "" {
		apiKey = "csghub-lite"
	}

	models := strings.Join(modelIDs, ",")
	if models == "" {
		models = modelID
	}

	onboardCtx, cancel := context.WithTimeout(ctx, csgclawOnboardTimeout)
	defer cancel()

	args := []string{
		"onboard",
		"--base-url", strings.TrimRight(serverURL, "/") + "/v1",
		"--api-key", apiKey,
		"--models", models,
	}

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
