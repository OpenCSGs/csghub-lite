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
	existing := make(map[string]string)
	existingSections := make(map[string]map[string]string)
	if data, err := os.ReadFile(configPath); err == nil {
		parseTomlFile(string(data), existing, existingSections)
	}

	// Update provider settings
	baseURL := strings.TrimRight(serverURL, "/") + "/v1"
	existing["model_provider"] = ProviderID
	existing["model"] = strings.TrimSpace(selectedModelID)

	// Write model_providers section
	if existingSections["model_providers"] == nil {
		existingSections["model_providers"] = make(map[string]string)
	}
	providerPrefix := fmt.Sprintf("model_providers.%s.", ProviderID)
	existingSections["model_providers"][providerPrefix+"name"] = "OpenCSG"
	existingSections["model_providers"][providerPrefix+"base_url"] = baseURL
	existingSections["model_providers"][providerPrefix+"api_key"] = strings.TrimSpace(apiKey)
	existingSections["model_providers"][providerPrefix+"supports_websockets"] = "false"

	// Write model catalog
	modelCatalogPath, err := writeModelCatalog(models)
	if err != nil {
		return err
	}
	existing["model_catalog_json"] = modelCatalogPath

	// Build TOML content
	var buf strings.Builder
	for key, value := range existing {
		buf.WriteString(fmt.Sprintf("%s = %q\n", key, value))
	}
	// Write model_providers section keys
	for sectionName, section := range existingSections {
		if sectionName == "model_providers" {
			for key, value := range section {
				buf.WriteString(fmt.Sprintf("%s = %q\n", key, value))
			}
		}
	}
	// Preserve project sections
	for sectionName, section := range existingSections {
		if strings.HasPrefix(sectionName, "projects.") {
			buf.WriteString(fmt.Sprintf("[%s]\n", sectionName))
			for key, value := range section {
				if !strings.HasPrefix(key, sectionName+".") {
					continue
				}
				shortKey := strings.TrimPrefix(key, sectionName+".")
				buf.WriteString(fmt.Sprintf("%s = %q\n", shortKey, value))
			}
		}
	}

	data := []byte(buf.String())
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(configPath, data, 0o644)
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

// parseTomlFile is a simple TOML parser that extracts key=value pairs and section headers.
func parseTomlFile(content string, kv map[string]string, sections map[string]map[string]string) {
	var currentSection string
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			currentSection = strings.Trim(line, "[]")
			if sections[currentSection] == nil {
				sections[currentSection] = make(map[string]string)
			}
			continue
		}
		if idx := strings.Index(line, "="); idx > 0 {
			key := strings.TrimSpace(line[:idx])
			value := strings.TrimSpace(line[idx+1:])
			// Remove quotes from value
			if strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"") {
				value = strings.Trim(value, "\"")
			}
			if currentSection != "" {
				// For sections like [projects."/path"], store as section.key
				fullKey := currentSection + "." + key
				if sections[currentSection] == nil {
					sections[currentSection] = make(map[string]string)
				}
				sections[currentSection][fullKey] = value
			} else {
				kv[key] = value
			}
		}
	}
}