package claudeagent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

const (
	ProviderID = "csghub-lite"
)

// SyncConfig persists Claude Code settings to ~/.claude/settings.json
// so that subsequent launches without csghub-lite will still use the
// configured API endpoint.
func SyncConfig(serverURL, apiKey, modelID string) error {
	settingsPath, err := SettingsPath()
	if err != nil {
		return err
	}

	return syncJSONFile(settingsPath, func(doc map[string]interface{}) {
		if modelID = strings.TrimSpace(modelID); modelID != "" {
			doc["model"] = modelID
		}
		env := ensureObject(doc, "env")
		env["ANTHROPIC_BASE_URL"] = strings.TrimRight(serverURL, "/")
		env["ANTHROPIC_API_KEY"] = strings.TrimSpace(apiKey)
		delete(env, "ANTHROPIC_AUTH_TOKEN")
		env["CLAUDE_API_BASE_URL"] = strings.TrimRight(serverURL, "/")
		env["CLAUDE_API_KEY"] = strings.TrimSpace(apiKey)
	})
}

// SettingsPath returns the path to Claude Code's settings.json.
func SettingsPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".claude", "settings.json"), nil
}

func syncJSONFile(path string, mutate func(map[string]interface{})) error {
	doc := map[string]interface{}{}
	if data, err := os.ReadFile(path); err == nil && len(data) > 0 {
		if err := json.Unmarshal(data, &doc); err != nil {
			// If the file is malformed, start fresh
			doc = map[string]interface{}{}
		}
	}

	mutate(doc)

	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func ensureObject(parent map[string]interface{}, key string) map[string]interface{} {
	if child, ok := parent[key].(map[string]interface{}); ok {
		return child
	}
	child := map[string]interface{}{}
	parent[key] = child
	return child
}
