package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/opencsgs/csghub-lite/pkg/api"
)

func TestResolveLaunchModelUsesServerDefault(t *testing.T) {
	server := launchModelTestServer([]api.ModelInfo{
		{Model: "Qwen/Qwen3.5-2B", Source: "local"},
		{Model: "Qwen/Qwen2.5-Coder-1.5B", Source: "local"},
	})
	defer server.Close()

	got, err := resolveLaunchModel(server.URL, "Qwen/Qwen2.5-Coder-1.5B", "", true, false)
	if err != nil {
		t.Fatalf("resolveLaunchModel returned error: %v", err)
	}
	if got != "Qwen/Qwen2.5-Coder-1.5B" {
		t.Fatalf("resolveLaunchModel chose %q, want server default coder model", got)
	}
}

func TestResolveLaunchModelAcceptsCloudModelID(t *testing.T) {
	server := launchModelTestServer([]api.ModelInfo{
		{Model: "Qwen/Qwen3.5-2B", Source: "local"},
		{Model: "minimax-m2.5", DisplayName: "MiniMax M2.5", Source: "cloud"},
	})
	defer server.Close()

	got, err := resolveLaunchModel(server.URL, "Qwen/Qwen3.5-2B", "minimax-m2.5", true, true)
	if err != nil {
		t.Fatalf("resolveLaunchModel returned error: %v", err)
	}
	if got != "minimax-m2.5" {
		t.Fatalf("resolveLaunchModel chose %q, want requested cloud model", got)
	}
}

func TestResolveLaunchModelMissingCloudTokenShowsSettingsHint(t *testing.T) {
	server := launchModelTestServer([]api.ModelInfo{
		{Model: "Qwen/Qwen3.5-2B", Source: "local"},
	})
	defer server.Close()

	_, err := resolveLaunchModel(server.URL, "Qwen/Qwen3.5-2B", "afrideva/Qwen2-0.5B-Instruct-GGUF:fh23aijhzx8g", true, false)
	if err == nil {
		t.Fatal("resolveLaunchModel returned nil error, want settings hint")
	}
	if got := err.Error(); !strings.Contains(got, "open csghub-lite Settings and save an Access Token first") {
		t.Fatalf("error = %q, want settings hint for missing cloud token", got)
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
      "opencsg": {
        "baseUrl": "http://127.0.0.1:11435/v1"
      }
    }
  },
  "agents": {
    "defaults": {
      "model": {
        "primary": "opencsg/Qwen/Qwen3.5-2B"
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

func TestSyncOpenClawProfileRewritesStaleModelCatalog(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	profileDir := filepath.Join(home, ".openclaw-"+openClawLaunchProfile)
	agentDir := filepath.Join(profileDir, "agents", "main", "agent")
	if err := os.MkdirAll(agentDir, 0o755); err != nil {
		t.Fatalf("mkdir agent dir: %v", err)
	}

	staleProfile := `{
  "models": {
    "mode": "merge",
    "providers": {
      "ollama": {
        "baseUrl": "http://127.0.0.1:11436",
        "models": [{"id": "old-local"}]
      },
      "csghub-lite-2": {
        "baseUrl": "http://127.0.0.1:11435/v1",
        "models": [{"id": "old-provider"}]
      },
      "csghub": {
        "baseUrl": "http://127.0.0.1:11435/v1",
        "models": [{"id": "old-cloud"}]
      }
    }
  },
  "agents": {
    "defaults": {
      "model": {
        "primary": "csghub/old-cloud"
      },
      "models": {
        "csghub/old-cloud": {}
      }
    }
  }
}`
	if err := os.WriteFile(filepath.Join(profileDir, "openclaw.json"), []byte(staleProfile), 0o644); err != nil {
		t.Fatalf("write stale profile: %v", err)
	}
	if err := os.WriteFile(filepath.Join(agentDir, "models.json"), []byte(`{"providers":{"csghub":{"models":[{"id":"old-cloud"}]}}}`), 0o644); err != nil {
		t.Fatalf("write stale agent models: %v", err)
	}

	models := []api.ModelInfo{
		{Model: "minimax-m2.5", DisplayName: "MiniMax M2.5", Source: "cloud"},
		{Model: "Qwen/Qwen3.5-2B", DisplayName: "Qwen/Qwen3.5-2B", Source: "local"},
	}
	if err := syncOpenClawProfile("http://127.0.0.1:11435", "user-token", "minimax-m2.5", models); err != nil {
		t.Fatalf("syncOpenClawProfile returned error: %v", err)
	}

	var profile struct {
		Models struct {
			Providers map[string]struct {
				BaseURL string `json:"baseUrl"`
				APIKey  string `json:"apiKey"`
				Models  []struct {
					ID string `json:"id"`
				} `json:"models"`
			} `json:"providers"`
		} `json:"models"`
		Agents struct {
			Defaults struct {
				Model struct {
					Primary string `json:"primary"`
				} `json:"model"`
				Models map[string]map[string]interface{} `json:"models"`
			} `json:"defaults"`
		} `json:"agents"`
	}
	data, err := os.ReadFile(filepath.Join(profileDir, "openclaw.json"))
	if err != nil {
		t.Fatalf("read synced profile: %v", err)
	}
	if err := json.Unmarshal(data, &profile); err != nil {
		t.Fatalf("decode synced profile: %v", err)
	}
	if len(profile.Models.Providers) != 1 {
		t.Fatalf("providers len = %d, want 1", len(profile.Models.Providers))
	}
	provider, ok := profile.Models.Providers[openClawLaunchProviderID]
	if !ok {
		t.Fatalf("provider %q missing after sync: %#v", openClawLaunchProviderID, profile.Models.Providers)
	}
	if provider.BaseURL != "http://127.0.0.1:11435/v1" {
		t.Fatalf("provider baseUrl = %q, want local v1 URL", provider.BaseURL)
	}
	if provider.APIKey != "user-token" {
		t.Fatalf("provider apiKey = %q, want saved user token", provider.APIKey)
	}
	if got := collectOpenClawModelIDs(provider.Models); !sameStrings(got, []string{"minimax-m2.5", "Qwen/Qwen3.5-2B"}) {
		t.Fatalf("provider model ids = %#v, want refreshed model ids", got)
	}
	if profile.Agents.Defaults.Model.Primary != "opencsg/minimax-m2.5" {
		t.Fatalf("primary model = %q, want refreshed cloud model", profile.Agents.Defaults.Model.Primary)
	}
	if got := mapKeys(profile.Agents.Defaults.Models); !sameStrings(got, []string{
		"opencsg/minimax-m2.5",
		"opencsg/Qwen/Qwen3.5-2B",
	}) {
		t.Fatalf("defaults.models = %#v, want refreshed managed models", got)
	}

	var agentModels struct {
		Providers map[string]struct {
			APIKey string `json:"apiKey"`
			Models []struct {
				ID string `json:"id"`
			} `json:"models"`
		} `json:"providers"`
	}
	data, err = os.ReadFile(filepath.Join(agentDir, "models.json"))
	if err != nil {
		t.Fatalf("read synced agent models: %v", err)
	}
	if err := json.Unmarshal(data, &agentModels); err != nil {
		t.Fatalf("decode synced agent models: %v", err)
	}
	if len(agentModels.Providers) != 1 {
		t.Fatalf("agent providers len = %d, want 1", len(agentModels.Providers))
	}
	if agentModels.Providers[openClawLaunchProviderID].APIKey != "user-token" {
		t.Fatalf("agent provider apiKey = %q, want saved user token", agentModels.Providers[openClawLaunchProviderID].APIKey)
	}
	if got := collectOpenClawModelIDs(agentModels.Providers[openClawLaunchProviderID].Models); !sameStrings(got, []string{"minimax-m2.5", "Qwen/Qwen3.5-2B"}) {
		t.Fatalf("agent model ids = %#v, want refreshed model ids", got)
	}
}

func TestClaudeLaunchSettingsJSONIncludesAcceptEditsMode(t *testing.T) {
	raw := claudeLaunchSettingsJSON("http://127.0.0.1:11435")

	var payload struct {
		Env         map[string]string `json:"env"`
		Permissions struct {
			DefaultMode string `json:"defaultMode"`
		} `json:"permissions"`
	}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		t.Fatalf("decode settings json: %v", err)
	}
	if payload.Env["ANTHROPIC_BASE_URL"] != "http://127.0.0.1:11435" {
		t.Fatalf("ANTHROPIC_BASE_URL = %q, want test server URL", payload.Env["ANTHROPIC_BASE_URL"])
	}
	if payload.Permissions.DefaultMode != "acceptEdits" {
		t.Fatalf("permissions.defaultMode = %q, want acceptEdits", payload.Permissions.DefaultMode)
	}
}

func launchModelTestServer(models []api.ModelInfo) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/tags":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(api.TagsResponse{Models: models})
		default:
			http.NotFound(w, r)
		}
	}))
}

func collectOpenClawModelIDs(items []struct {
	ID string `json:"id"`
}) []string {
	ids := make([]string, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID)
	}
	return ids
}

func mapKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	return keys
}

func sameStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	seen := make(map[string]int, len(want))
	for _, item := range want {
		seen[item]++
	}
	for _, item := range got {
		seen[item]--
		if seen[item] < 0 {
			return false
		}
	}
	for _, count := range seen {
		if count != 0 {
			return false
		}
	}
	return true
}
