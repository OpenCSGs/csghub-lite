package claudeagent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestSyncConfigWritesEnvSettings(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	err := SyncConfig("http://127.0.0.1:11435", "test-token")
	if err != nil {
		t.Fatalf("SyncConfig returned error: %v", err)
	}

	settingsPath := filepath.Join(home, ".claude", "settings.json")
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("read settings: %v", err)
	}

	var settings struct {
		Env map[string]string `json:"env"`
	}
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatalf("decode settings: %v", err)
	}

	if settings.Env["ANTHROPIC_BASE_URL"] != "http://127.0.0.1:11435" {
		t.Fatalf("ANTHROPIC_BASE_URL = %q, want http://127.0.0.1:11435", settings.Env["ANTHROPIC_BASE_URL"])
	}
	if settings.Env["ANTHROPIC_API_KEY"] != "test-token" {
		t.Fatalf("ANTHROPIC_API_KEY = %q, want test-token", settings.Env["ANTHROPIC_API_KEY"])
	}
	if settings.Env["CLAUDE_API_BASE_URL"] != "http://127.0.0.1:11435" {
		t.Fatalf("CLAUDE_API_BASE_URL = %q, want http://127.0.0.1:11435", settings.Env["CLAUDE_API_BASE_URL"])
	}
}

func TestSyncConfigPreservesExistingSettings(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	claudeDir := filepath.Join(home, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatalf("mkdir claude dir: %v", err)
	}
	existing := `{"theme": "dark", "env": {"OTHER_VAR": "other-value"}}`
	if err := os.WriteFile(filepath.Join(claudeDir, "settings.json"), []byte(existing), 0o644); err != nil {
		t.Fatalf("write existing settings: %v", err)
	}

	err := SyncConfig("http://127.0.0.1:11435", "test-token")
	if err != nil {
		t.Fatalf("SyncConfig returned error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(claudeDir, "settings.json"))
	if err != nil {
		t.Fatalf("read settings: %v", err)
	}

	var settings struct {
		Theme string            `json:"theme"`
		Env   map[string]string `json:"env"`
	}
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatalf("decode settings: %v", err)
	}

	if settings.Theme != "dark" {
		t.Fatalf("theme = %q, want dark", settings.Theme)
	}
	if settings.Env["OTHER_VAR"] != "other-value" {
		t.Fatalf("OTHER_VAR = %q, want other-value", settings.Env["OTHER_VAR"])
	}
	if settings.Env["ANTHROPIC_BASE_URL"] != "http://127.0.0.1:11435" {
		t.Fatalf("ANTHROPIC_BASE_URL = %q, want http://127.0.0.1:11435", settings.Env["ANTHROPIC_BASE_URL"])
	}
}
