package server

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/opencsgs/csghub-lite/internal/config"
)

func TestExtractDashboardURL(t *testing.T) {
	output := []byte("Using model Qwen/Qwen3.5-2B\nDashboard URL: http://127.0.0.1:18789/#token=abc123\nBrowser launch disabled (--no-open).\n")

	got, err := extractDashboardURL(output)
	if err != nil {
		t.Fatalf("extractDashboardURL returned error: %v", err)
	}
	if want := "http://127.0.0.1:18789/#token=abc123"; got != want {
		t.Fatalf("extractDashboardURL = %q, want %q", got, want)
	}
}

func TestDashboardHostPort(t *testing.T) {
	got, err := dashboardHostPort("http://127.0.0.1:18789/#token=abc123")
	if err != nil {
		t.Fatalf("dashboardHostPort returned error: %v", err)
	}
	if want := "127.0.0.1:18789"; got != want {
		t.Fatalf("dashboardHostPort = %q, want %q", got, want)
	}
}

func TestOpenClawDirectChatURL(t *testing.T) {
	got, err := openClawDirectChatURL("http://127.0.0.1:18789/#token=abc123", "main")
	if err != nil {
		t.Fatalf("openClawDirectChatURL returned error: %v", err)
	}
	if want := "http://127.0.0.1:18789/chat?session=main#token=abc123"; got != want {
		t.Fatalf("openClawDirectChatURL = %q, want %q", got, want)
	}
}

func TestOpenClawProfileMatches(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	cfgDir := filepath.Join(home, ".openclaw-"+openClawWebProfile)
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}

	configJSON := `{
  "models": {
    "providers": {
      "csghub": {
        "baseUrl": "http://127.0.0.1:11435/v1"
      }
    }
  },
  "agents": {
    "defaults": {
      "model": {
        "primary": "csghub/Qwen/Qwen3.5-2B"
      }
    }
  }
}`
	if err := os.WriteFile(filepath.Join(cfgDir, "openclaw.json"), []byte(configJSON), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	ok, err := openClawProfileMatches("http://127.0.0.1:11435", "Qwen/Qwen3.5-2B")
	if err != nil {
		t.Fatalf("openClawProfileMatches returned error: %v", err)
	}
	if !ok {
		t.Fatal("openClawProfileMatches returned false, want true")
	}
}

func TestLocalBaseURLDefaultsToConfigListenAddr(t *testing.T) {
	s := &Server{cfg: &config.Config{}}

	if got := s.localBaseURL(); got != "http://127.0.0.1:11435" {
		t.Fatalf("localBaseURL = %q, want %q", got, "http://127.0.0.1:11435")
	}
}
