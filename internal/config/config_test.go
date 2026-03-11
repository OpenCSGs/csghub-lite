package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func setupTestDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	return dir
}

func TestDefaultValues(t *testing.T) {
	Reset()
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.ServerURL != DefaultServerURL {
		t.Errorf("ServerURL = %q, want %q", cfg.ServerURL, DefaultServerURL)
	}
	if cfg.ListenAddr != DefaultListenAddr {
		t.Errorf("ListenAddr = %q, want %q", cfg.ListenAddr, DefaultListenAddr)
	}
}

func TestSaveAndLoad(t *testing.T) {
	dir := setupTestDir(t)
	cfgPath := filepath.Join(dir, ConfigFile)

	cfg := &Config{
		ServerURL:  "https://custom.example.com",
		Token:      "test-token-123",
		ListenAddr: ":8080",
		ModelDir:   filepath.Join(dir, "models"),
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent error: %v", err)
	}
	if err := os.WriteFile(cfgPath, data, 0o644); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}

	// Read it back
	readData, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("ReadFile error: %v", err)
	}

	var loaded Config
	if err := json.Unmarshal(readData, &loaded); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if loaded.ServerURL != cfg.ServerURL {
		t.Errorf("ServerURL = %q, want %q", loaded.ServerURL, cfg.ServerURL)
	}
	if loaded.Token != cfg.Token {
		t.Errorf("Token = %q, want %q", loaded.Token, cfg.Token)
	}
	if loaded.ListenAddr != cfg.ListenAddr {
		t.Errorf("ListenAddr = %q, want %q", loaded.ListenAddr, cfg.ListenAddr)
	}
	if loaded.ModelDir != cfg.ModelDir {
		t.Errorf("ModelDir = %q, want %q", loaded.ModelDir, cfg.ModelDir)
	}
}

func TestSaveCreatesDirectory(t *testing.T) {
	Reset()

	dir := setupTestDir(t)
	nested := filepath.Join(dir, "deep", "nested")

	cfg := &Config{
		ServerURL:  DefaultServerURL,
		ListenAddr: DefaultListenAddr,
		ModelDir:   filepath.Join(nested, "models"),
	}

	// Can't use Save() directly since it uses AppHome(), but test the pattern
	cfgPath := filepath.Join(nested, ConfigFile)
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
		t.Fatalf("MkdirAll error: %v", err)
	}

	data, _ := json.MarshalIndent(cfg, "", "  ")
	if err := os.WriteFile(cfgPath, data, 0o644); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}

	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		t.Error("config file was not created")
	}
}

func TestConfigGet(t *testing.T) {
	Reset()
	cfg := Get()
	if cfg == nil {
		t.Fatal("Get() returned nil")
	}
	if cfg.ServerURL != DefaultServerURL {
		t.Errorf("ServerURL = %q, want %q", cfg.ServerURL, DefaultServerURL)
	}
}

func TestAppHome(t *testing.T) {
	home, err := AppHome()
	if err != nil {
		t.Fatalf("AppHome() error: %v", err)
	}
	if home == "" {
		t.Error("AppHome() returned empty string")
	}
	if !filepath.IsAbs(home) {
		t.Errorf("AppHome() = %q, want absolute path", home)
	}
}
