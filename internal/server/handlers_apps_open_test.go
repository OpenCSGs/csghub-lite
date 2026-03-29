package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	neturl "net/url"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/opencsgs/csghub-lite/internal/cloud"
	"github.com/opencsgs/csghub-lite/internal/config"
	"github.com/opencsgs/csghub-lite/internal/model"
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

func TestRefreshOpenClawModelCatalogRefreshesCloudCache(t *testing.T) {
	currentModel := "stale/model"
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"object": "list",
			"data": []map[string]any{
				{
					"id":           currentModel,
					"task":         "text-generation",
					"display_name": currentModel,
				},
			},
		})
	}))
	defer apiServer.Close()

	s := &Server{
		cfg:   &config.Config{Token: "test-token"},
		cloud: cloud.NewService(apiServer.URL),
	}

	models, err := s.cloud.ListChatModels(context.Background())
	if err != nil {
		t.Fatalf("ListChatModels returned error: %v", err)
	}
	if len(models) != 1 || models[0].Model != "stale/model" {
		t.Fatalf("initial models = %#v, want stale/model", models)
	}

	currentModel = "fresh/model"
	s.refreshOpenClawModelCatalog(context.Background())

	models, err = s.cloud.ListChatModels(context.Background())
	if err != nil {
		t.Fatalf("ListChatModels after refresh returned error: %v", err)
	}
	if len(models) != 1 || models[0].Model != "fresh/model" {
		t.Fatalf("models after refresh = %#v, want fresh/model", models)
	}
}

func TestOpenAIAppShellURLReturnsShellPage(t *testing.T) {
	cfg := &config.Config{ModelDir: t.TempDir(), ListenAddr: ":11435"}
	if err := model.SaveManifest(cfg.ModelDir, &model.LocalModel{
		Namespace:    "Qwen",
		Name:         "Qwen3.5-2B",
		Format:       model.FormatGGUF,
		Size:         4_000_000_000,
		Files:        []string{"model.gguf"},
		DownloadedAt: time.Unix(123, 0),
	}); err != nil {
		t.Fatalf("save model manifest: %v", err)
	}

	binDir := t.TempDir()
	commandPath := filepath.Join(binDir, "claude")
	content := "#!/bin/sh\nexit 0\n"
	if runtime.GOOS == "windows" {
		commandPath = filepath.Join(binDir, "claude.cmd")
		content = "@echo off\r\nexit /b 0\r\n"
	}
	if err := os.WriteFile(commandPath, []byte(content), 0o755); err != nil {
		t.Fatalf("write fake binary: %v", err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	s := New(cfg, "test")

	url, err := s.openAIAppShellURL(context.Background(), "claude-code", "", "")
	if err != nil {
		t.Fatalf("openAIAppShellURL returned error: %v", err)
	}

	parsed, err := neturl.Parse(url)
	if err != nil {
		t.Fatalf("parse url: %v", err)
	}
	if parsed.Path != "/ai-apps/shell" {
		t.Fatalf("path = %q, want /ai-apps/shell", parsed.Path)
	}
	sessionID := parsed.Query().Get("session_id")
	if sessionID == "" {
		t.Fatal("expected session_id in shell url")
	}
	if !s.appShells.Close(sessionID) {
		t.Fatalf("expected session %q to exist", sessionID)
	}
}

func TestOpenAIAppShellURLUsesRequestedWorkDir(t *testing.T) {
	cfg := &config.Config{ModelDir: t.TempDir(), ListenAddr: ":11435"}
	if err := model.SaveManifest(cfg.ModelDir, &model.LocalModel{
		Namespace:    "Qwen",
		Name:         "Qwen3.5-2B",
		Format:       model.FormatGGUF,
		Size:         4_000_000_000,
		Files:        []string{"model.gguf"},
		DownloadedAt: time.Unix(123, 0),
	}); err != nil {
		t.Fatalf("save model manifest: %v", err)
	}

	binDir := t.TempDir()
	commandPath := filepath.Join(binDir, "claude")
	content := "#!/bin/sh\nexit 0\n"
	if runtime.GOOS == "windows" {
		commandPath = filepath.Join(binDir, "claude.cmd")
		content = "@echo off\r\nexit /b 0\r\n"
	}
	if err := os.WriteFile(commandPath, []byte(content), 0o755); err != nil {
		t.Fatalf("write fake binary: %v", err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	workDir := t.TempDir()
	s := New(cfg, "test")

	url, err := s.openAIAppShellURL(context.Background(), "claude-code", "", workDir)
	if err != nil {
		t.Fatalf("openAIAppShellURL returned error: %v", err)
	}

	parsed, err := neturl.Parse(url)
	if err != nil {
		t.Fatalf("parse url: %v", err)
	}
	sessionID := parsed.Query().Get("session_id")
	if sessionID == "" {
		t.Fatal("expected session_id in shell url")
	}
	session, ok := s.appShells.Get(sessionID)
	if !ok {
		t.Fatalf("expected session %q to exist", sessionID)
	}
	if session.workDir != workDir {
		t.Fatalf("session workDir = %q, want %q", session.workDir, workDir)
	}
	_ = s.appShells.Close(sessionID)
}

func TestWriteOpenCodeWebLaunchConfigIncludesAllModels(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	path, err := writeOpenCodeWebLaunchConfig("http://127.0.0.1:11435", "Qwen/Qwen3.5-2B", []string{
		"Qwen/Qwen3.5-2B",
		"Qwen/Qwen2.5-Coder-1.5B",
	})
	if err != nil {
		t.Fatalf("writeOpenCodeWebLaunchConfig returned error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("decode config: %v", err)
	}

	providers, ok := payload["provider"].(map[string]interface{})
	if !ok {
		t.Fatalf("provider field missing or invalid: %#v", payload["provider"])
	}
	provider, ok := providers["csghub-lite"].(map[string]interface{})
	if !ok {
		t.Fatalf("csghub-lite provider missing: %#v", providers)
	}
	models, ok := provider["models"].(map[string]interface{})
	if !ok {
		t.Fatalf("provider models missing: %#v", provider["models"])
	}
	if len(models) != 2 {
		t.Fatalf("models count = %d, want 2 (%#v)", len(models), models)
	}
	if _, ok := models["Qwen/Qwen2.5-Coder-1.5B"]; !ok {
		t.Fatalf("missing coder model in config: %#v", models)
	}
}
