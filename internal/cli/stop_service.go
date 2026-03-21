package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/opencsgs/csghub-lite/internal/config"
	"github.com/spf13/cobra"
)

func newStopServiceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "stop-service",
		Aliases: []string{"stop-server", "down"},
		Short:   "Stop the background csghub-lite service",
		Long:    "Stop the background csghub-lite API service started by 'serve' or auto-started by client commands.",
		Args:    cobra.NoArgs,
		RunE:    runStopService,
	}
	return cmd
}

func runStopService(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	baseURL := serverBaseURL(cfg)
	if serverHealthy(baseURL) {
		if err := requestServerShutdown(baseURL); err != nil {
			return err
		}
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			if !serverHealthy(baseURL) {
				_ = removePIDFile()
				fmt.Println("Stopped csghub-lite service")
				return nil
			}
			time.Sleep(200 * time.Millisecond)
		}
		return fmt.Errorf("service did not stop within 5s")
	}

	pid := ServerPID()
	if pid <= 0 {
		return fmt.Errorf("no running csghub-lite service found")
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		_ = removePIDFile()
		return fmt.Errorf("finding server process %d: %w", pid, err)
	}

	if !processExists(proc) {
		_ = removePIDFile()
		fmt.Printf("csghub-lite service is already stopped (pid %d)\n", pid)
		return nil
	}
	if err := stopProcess(proc); err != nil {
		if !processExists(proc) {
			_ = removePIDFile()
			fmt.Printf("csghub-lite service is already stopped (pid %d)\n", pid)
			return nil
		}
		return fmt.Errorf("stopping service pid %d: %w", pid, err)
	}

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if !serverHealthy(baseURL) && !processExists(proc) {
			_ = removePIDFile()
			fmt.Printf("Stopped csghub-lite service (pid %d)\n", pid)
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}

	if !serverHealthy(baseURL) {
		_ = removePIDFile()
		fmt.Printf("Stopped csghub-lite service (pid %d)\n", pid)
		return nil
	}

	return fmt.Errorf("service pid %d did not stop within 5s", pid)
}

func requestServerShutdown(baseURL string) error {
	client := &http.Client{Timeout: 3 * time.Second}
	body, _ := json.Marshal(map[string]bool{"shutdown": true})
	resp, err := client.Post(baseURL+"/api/shutdown", "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("requesting server shutdown: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("shutdown request failed: HTTP %d", resp.StatusCode)
	}
	return nil
}
