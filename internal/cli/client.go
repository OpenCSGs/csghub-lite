package cli

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/opencsgs/csghub-lite/internal/config"
)

// ensureServer makes sure a csghub-lite API server is running and returns
// its base URL. If no server is reachable, it spawns one in the background.
func ensureServer(cfg *config.Config) (string, error) {
	baseURL := serverBaseURL(cfg)

	if serverHealthy(baseURL) {
		return baseURL, nil
	}

	if err := startBackgroundServer(cfg); err != nil {
		return "", fmt.Errorf("starting background server: %w", err)
	}

	if err := waitForServer(baseURL, 15*time.Second); err != nil {
		return "", err
	}

	return baseURL, nil
}

func serverBaseURL(cfg *config.Config) string {
	addr := cfg.ListenAddr
	if strings.HasPrefix(addr, ":") {
		addr = "127.0.0.1" + addr
	}
	return "http://" + addr
}

func serverHealthy(baseURL string) bool {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(baseURL + "/api/health")
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func startBackgroundServer(cfg *config.Config) error {
	self, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolving executable path: %w", err)
	}

	args := []string{"serve"}
	if cfg.ListenAddr != config.DefaultListenAddr {
		args = append(args, "--listen", cfg.ListenAddr)
	}

	cmd := exec.Command(self, args...)
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil

	detachProcess(cmd)

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("exec serve: %w", err)
	}

	if err := writePIDFile(cmd.Process.Pid); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not write PID file: %v\n", err)
	}

	return nil
}

func waitForServer(baseURL string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if serverHealthy(baseURL) {
			return nil
		}
		time.Sleep(250 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for csghub-lite server at %s", baseURL)
}

func pidFilePath() (string, error) {
	home, err := config.AppHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "server.pid"), nil
}

func writePIDFile(pid int) error {
	path, err := pidFilePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(strconv.Itoa(pid)), 0o644)
}

// ServerPID reads the stored server PID, or returns 0 if unavailable.
func ServerPID() int {
	path, err := pidFilePath()
	if err != nil {
		return 0
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	pid, _ := strconv.Atoi(strings.TrimSpace(string(data)))
	return pid
}

// detachProcess is defined per-platform in client_unix.go / client_windows.go.
