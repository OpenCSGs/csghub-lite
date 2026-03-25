package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/opencsgs/csghub-lite/internal/config"
	"github.com/opencsgs/csghub-lite/internal/model"
)

func TestResolveLaunchModelPrefersCoder(t *testing.T) {
	cfg := &config.Config{ModelDir: t.TempDir()}

	if err := model.SaveManifest(cfg.ModelDir, &model.LocalModel{
		Namespace:    "Qwen",
		Name:         "Qwen3.5-2B",
		Format:       model.FormatGGUF,
		Size:         4_000_000_000,
		Files:        []string{"model.gguf"},
		DownloadedAt: time.Now().Add(-time.Hour),
	}); err != nil {
		t.Fatalf("save general model: %v", err)
	}
	if err := model.SaveManifest(cfg.ModelDir, &model.LocalModel{
		Namespace:    "Qwen",
		Name:         "Qwen2.5-Coder-1.5B",
		Format:       model.FormatGGUF,
		Size:         2_000_000_000,
		Files:        []string{"model.gguf"},
		DownloadedAt: time.Now(),
	}); err != nil {
		t.Fatalf("save coder model: %v", err)
	}

	got, err := resolveLaunchModel(cfg, "", true)
	if err != nil {
		t.Fatalf("resolveLaunchModel returned error: %v", err)
	}
	if got != "Qwen/Qwen2.5-Coder-1.5B" {
		t.Fatalf("resolveLaunchModel chose %q, want coder model", got)
	}
}

func TestPrependArgsIfMissing(t *testing.T) {
	args := prependArgsIfMissing([]string{"run", "hello"}, []string{"--model", "demo"}, "--model", "-m")
	if len(args) != 4 || args[0] != "--model" || args[1] != "demo" {
		t.Fatalf("prependArgsIfMissing prepended unexpected args: %#v", args)
	}

	unchanged := prependArgsIfMissing([]string{"--model", "other", "run"}, []string{"--model", "demo"}, "--model", "-m")
	if len(unchanged) != 3 || unchanged[0] != "--model" || unchanged[1] != "other" {
		t.Fatalf("prependArgsIfMissing should not duplicate model flags: %#v", unchanged)
	}
}

func TestWriteOpenCodeLaunchConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	path, err := writeOpenCodeLaunchConfig("http://127.0.0.1:11435", "Qwen/Qwen3.5-2B")
	if err != nil {
		t.Fatalf("writeOpenCodeLaunchConfig returned error: %v", err)
	}
	if filepath.Base(path) != "opencode.json" {
		t.Fatalf("unexpected config filename: %s", path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("decode config: %v", err)
	}

	if payload["model"] != "csghub-lite/Qwen/Qwen3.5-2B" {
		t.Fatalf("unexpected model field: %#v", payload["model"])
	}

	providers, ok := payload["provider"].(map[string]interface{})
	if !ok {
		t.Fatalf("provider field missing or invalid: %#v", payload["provider"])
	}
	provider, ok := providers["csghub-lite"].(map[string]interface{})
	if !ok {
		t.Fatalf("csghub-lite provider missing: %#v", providers)
	}
	options, ok := provider["options"].(map[string]interface{})
	if !ok || options["baseURL"] != "http://127.0.0.1:11435/v1" {
		t.Fatalf("unexpected provider options: %#v", provider["options"])
	}
}

func TestOpenClawProfileMatches(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	cfgDir := filepath.Join(home, ".openclaw-"+openClawLaunchProfile)
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}

	configJSON := `{
  "models": {
    "providers": {
      "csghub-lite": {
        "baseUrl": "http://127.0.0.1:11435/v1"
      }
    }
  },
  "agents": {
    "defaults": {
      "model": {
        "primary": "csghub-lite/Qwen/Qwen3.5-2B"
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
