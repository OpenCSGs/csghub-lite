package codexagent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/opencsgs/csghub-lite/pkg/api"
)

const (
	ProviderID = "csghub_lite"
)

// SyncConfig writes Codex configuration to ~/.codex/config.toml
// so subsequent launches use csghub-lite as the model provider.
func SyncConfig(serverURL, apiKey, selectedModelID string, models []api.ModelInfo) error {
	if strings.TrimSpace(selectedModelID) == "" && len(models) > 0 {
		selectedModelID = models[0].Model
	}

	configPath, err := ConfigPath()
	if err != nil {
		return err
	}

	// Read existing config to preserve user settings like trust_level
	existing := make(map[string]tomlValue)
	if data, err := os.ReadFile(configPath); err == nil {
		parseTomlFile(string(data), existing)
	}

	// Update provider settings
	baseURL := strings.TrimRight(serverURL, "/") + "/v1"
	existing["model_provider"] = stringVal(ProviderID)
	existing["model"] = stringVal(strings.TrimSpace(selectedModelID))
	existing["model_providers."+ProviderID+".name"] = stringVal("OpenCSG")
	existing["model_providers."+ProviderID+".base_url"] = stringVal(baseURL)
	existing["model_providers."+ProviderID+".api_key"] = stringVal(strings.TrimSpace(apiKey))
	existing["model_providers."+ProviderID+".supports_websockets"] = boolVal(false)

	// Write model catalog
	modelCatalogPath, err := writeModelCatalog(models)
	if err != nil {
		return err
	}
	existing["model_catalog_json"] = stringVal(modelCatalogPath)

	// Build TOML content, grouping dotted keys into sections
	var buf strings.Builder
	topLevel := make(map[string]tomlValue)
	sections := make(map[string]map[string]tomlValue)

	for key, value := range existing {
		// Check if this is a dotted key like "model_providers.csghub_lite.name"
		parts := strings.Split(key, ".")
		if len(parts) >= 2 && (parts[0] == "model_providers" || parts[0] == "projects") {
			sectionName := parts[0]
			if len(parts) >= 2 {
				// For model_providers.X.name, section is [model_providers.X]
				sectionName = parts[0] + "." + parts[1]
			}
			if sections[sectionName] == nil {
				sections[sectionName] = make(map[string]tomlValue)
			}
			// Key within section is the remaining parts
			sectionKey := strings.Join(parts[2:], ".")
			if len(parts) == 2 {
				// For model_providers.X, the key is just parts[1], but this shouldn't happen
				// Actually for model_provider (single key), it's top-level
				sectionKey = parts[1]
			}
			sections[sectionName][sectionKey] = value
		} else {
			topLevel[key] = value
		}
	}

	// Write top-level keys
	for key, value := range topLevel {
		buf.WriteString(formatTomlKV(key, value))
	}

	// Write sections
	for sectionName, section := range sections {
		buf.WriteString(fmt.Sprintf("[%s]\n", sectionName))
		for key, value := range section {
			buf.WriteString(formatTomlKV(key, value))
		}
	}

	data := []byte(buf.String())
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(configPath, data, 0o644)
}

type tomlValue struct {
	isBool   bool
	boolVal  bool
	strVal   string
}

func stringVal(s string) tomlValue {
	return tomlValue{strVal: s}
}

func boolVal(b bool) tomlValue {
	return tomlValue{isBool: true, boolVal: b}
}

func formatTomlKV(key string, value tomlValue) string {
	if value.isBool {
		return fmt.Sprintf("%s = %v\n", key, value.boolVal)
	}
	return fmt.Sprintf("%s = %q\n", key, value.strVal)
}

// ConfigPath returns the path to Codex config.toml.
func ConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".codex", "config.toml"), nil
}

func writeModelCatalog(models []api.ModelInfo) (string, error) {
	configDir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	catalogDir := filepath.Join(configDir, "csghub-lite")
	if err := os.MkdirAll(catalogDir, 0o755); err != nil {
		return "", err
	}

	entries := make([]modelCatalogEntry, 0, len(models))
	for _, m := range models {
		modelID := strings.TrimSpace(m.Model)
		if modelID == "" {
			continue
		}
		entries = append(entries, modelCatalogEntry{
			Slug:                       modelID,
			DisplayName:                modelID,
			Description:                "Model served by csghub-lite.",
			SupportedReasoningLevels:   []reasoningEffortPreset{},
			ShellType:                  "shell_command",
			Visibility:                 "list",
			SupportedInAPI:             true,
			Priority:                   len(entries),
			BaseInstructions:           "You are Codex, a coding agent. You and the user share the same workspace and collaborate to achieve the user's goals. Focus on practical, safe, concise help for software tasks.",
			SupportsReasoningSummaries: false,
			SupportVerbosity:           false,
			TruncationPolicy: truncationPolicy{
				Mode:  "bytes",
				Limit: 10000,
			},
			SupportsParallelToolCalls:  false,
			ExperimentalSupportedTools: []string{},
			InputModalities:            []string{"text"},
			ContextWindow:              m.ContextWindow,
		})
	}

	catalog := modelCatalog{Models: entries}
	data, err := json.MarshalIndent(catalog, "", "  ")
	if err != nil {
		return "", err
	}

	path := filepath.Join(catalogDir, "models.json")
	return path, os.WriteFile(path, data, 0o644)
}

func ConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".codex"), nil
}

type modelCatalog struct {
	Models []modelCatalogEntry `json:"models"`
}

type modelCatalogEntry struct {
	Slug                       string                       `json:"slug"`
	DisplayName                string                       `json:"display_name"`
	Description                string                       `json:"description"`
	SupportedReasoningLevels   []reasoningEffortPreset      `json:"supported_reasoning_levels"`
	ShellType                  string                       `json:"shell_type"`
	Visibility                 string                       `json:"visibility"`
	SupportedInAPI             bool                         `json:"supported_in_api"`
	Priority                   int                          `json:"priority"`
	BaseInstructions           string                       `json:"base_instructions"`
	SupportsReasoningSummaries bool                         `json:"supports_reasoning_summaries"`
	SupportVerbosity           bool                         `json:"support_verbosity"`
	TruncationPolicy           truncationPolicy             `json:"truncation_policy"`
	SupportsParallelToolCalls  bool                         `json:"supports_parallel_tool_calls"`
	ExperimentalSupportedTools []string                     `json:"experimental_supported_tools"`
	InputModalities            []string                     `json:"input_modalities,omitempty"`
	ContextWindow              int64                        `json:"context_window,omitempty"`
}

type reasoningEffortPreset struct {
	Effort      string `json:"effort"`
	Description string `json:"description"`
}

type truncationPolicy struct {
	Mode  string `json:"mode"`
	Limit int64  `json:"limit"`
}

// parseTomlFile is a simple TOML parser that extracts key=value pairs.
// Dotted keys like model_providers.X.name are stored with their full path.
func parseTomlFile(content string, kv map[string]tomlValue) {
	var currentSection string
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			currentSection = strings.Trim(line, "[]")
			continue
		}
		if idx := strings.Index(line, "="); idx > 0 {
			key := strings.TrimSpace(line[:idx])
			value := strings.TrimSpace(line[idx+1:])
			// Parse the value
			if value == "true" {
				if currentSection != "" {
					kv[currentSection+"."+key] = boolVal(true)
				} else {
					kv[key] = boolVal(true)
				}
			} else if value == "false" {
				if currentSection != "" {
					kv[currentSection+"."+key] = boolVal(false)
				} else {
					kv[key] = boolVal(false)
				}
			} else {
				// Remove quotes from string value
				if strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"") {
					value = strings.Trim(value, "\"")
				}
				if currentSection != "" {
					kv[currentSection+"."+key] = stringVal(value)
				} else {
					kv[key] = stringVal(value)
				}
			}
		}
	}
}