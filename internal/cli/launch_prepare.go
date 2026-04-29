package cli

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/opencsgs/csghub-lite/internal/config"
	"github.com/opencsgs/csghub-lite/pkg/api"
)

const (
	openCodeLaunchProviderID  = "csghub-lite"
	openClawLaunchProfile     = "csghub-lite"
	openClawLaunchProviderID  = "opencsg"
	openClawLaunchProviderAPI = "openai-completions"
	openClawConfigureTimeout  = 2 * time.Minute
	openClawContextWindow     = 16000
	openClawMaxTokens         = 4096
	openClawDefaultRegistry   = "https://registry.npmmirror.com"
	csgClawLaunchProviderID   = "csghub-lite"
	csgClawConfigureTimeout   = 2 * time.Minute
	csgClawManagerImage       = "opencsg-registry.cn-beijing.cr.aliyuncs.com/opencsghq/picoclaw:2026.4.26"
	codexCloudContextWindow   = 272000
	codexLocalContextWindow   = 8192
	codexBaseInstructions     = "You are Codex, a coding agent. You and the user share the same workspace and collaborate to achieve the user's goals. Focus on practical, safe, concise help for software tasks."
)

type preparedLaunch struct {
	Binary string
	Args   []string
	Env    []string
}

type codexModelCatalog struct {
	Models []codexModelCatalogEntry `json:"models"`
}

type codexModelCatalogEntry struct {
	Slug                       string                       `json:"slug"`
	DisplayName                string                       `json:"display_name"`
	Description                string                       `json:"description"`
	SupportedReasoningLevels   []codexReasoningEffortPreset `json:"supported_reasoning_levels"`
	ShellType                  string                       `json:"shell_type"`
	Visibility                 string                       `json:"visibility"`
	SupportedInAPI             bool                         `json:"supported_in_api"`
	Priority                   int                          `json:"priority"`
	BaseInstructions           string                       `json:"base_instructions"`
	SupportsReasoningSummaries bool                         `json:"supports_reasoning_summaries"`
	SupportVerbosity           bool                         `json:"support_verbosity"`
	TruncationPolicy           codexTruncationPolicy        `json:"truncation_policy"`
	SupportsParallelToolCalls  bool                         `json:"supports_parallel_tool_calls"`
	ExperimentalSupportedTools []string                     `json:"experimental_supported_tools"`
	InputModalities            []string                     `json:"input_modalities,omitempty"`
	ContextWindow              int64                        `json:"context_window,omitempty"`
}

type codexReasoningEffortPreset struct {
	Effort      string `json:"effort"`
	Description string `json:"description"`
}

type codexTruncationPolicy struct {
	Mode  string `json:"mode"`
	Limit int64  `json:"limit"`
}

func resolveLaunchModel(serverURL, defaultModel, requested string, skipPrompt, hasCloudToken bool) (string, error) {
	models, err := getLaunchModels(serverURL)
	if err != nil {
		return "", err
	}
	if len(models) == 0 {
		return "", fmt.Errorf("no models are currently available for AI apps")
	}

	choices := normalizeLaunchModelChoices(models)
	if len(choices) == 0 {
		return "", fmt.Errorf("no models are currently available for AI apps")
	}

	if requested != "" {
		for _, candidate := range choices {
			if candidate.ID == requested {
				return candidate.ID, nil
			}
		}
		if !hasCloudToken {
			return "", fmt.Errorf("model %q is not available for AI apps. If you are trying to use an OpenCSG model, please open csghub-lite Settings and save an Access Token first", requested)
		}
		return "", fmt.Errorf("model %q is not available for AI apps", requested)
	}

	defaultModel = strings.TrimSpace(defaultModel)
	if defaultModel != "" {
		for _, candidate := range choices {
			if candidate.ID == defaultModel {
				if len(choices) == 1 || skipPrompt || !stdinIsTerminal() {
					return candidate.ID, nil
				}
				return promptForLaunchModel(choices, candidate.ID)
			}
		}
	}

	if len(choices) == 1 || skipPrompt || !stdinIsTerminal() {
		return choices[0].ID, nil
	}

	return promptForLaunchModel(choices, "")
}

type launchModelChoice struct {
	ID    string
	Label string
}

func normalizeLaunchModelChoices(models []api.ModelInfo) []launchModelChoice {
	seen := make(map[string]struct{}, len(models))
	choices := make([]launchModelChoice, 0, len(models))
	for _, item := range models {
		modelID := strings.TrimSpace(item.Model)
		if modelID == "" {
			continue
		}
		if _, ok := seen[modelID]; ok {
			continue
		}
		seen[modelID] = struct{}{}

		label := modelID
		displayName := strings.TrimSpace(item.DisplayName)
		if displayName != "" && displayName != modelID {
			label = displayName + " [" + modelID + "]"
		}
		source := strings.TrimSpace(item.Source)
		if source != "" {
			label += " (" + source + ")"
		}
		choices = append(choices, launchModelChoice{
			ID:    modelID,
			Label: label,
		})
	}
	return choices
}

func stdinIsTerminal() bool {
	info, err := os.Stdin.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

func promptForLaunchModel(models []launchModelChoice, defaultModel string) (string, error) {
	fmt.Fprintln(os.Stderr, "Select a model for AI apps:")
	defaultIndex := 0
	if defaultModel != "" {
		for i, candidate := range models {
			if candidate.ID == defaultModel {
				defaultIndex = i
				break
			}
		}
	}
	for i, candidate := range models {
		label := candidate.Label
		if i == defaultIndex {
			label += " (default)"
		}
		fmt.Fprintf(os.Stderr, "  %d. %s\n", i+1, label)
	}
	fmt.Fprintf(os.Stderr, "Model [1-%d, default 1]: ", len(models))

	scanner := bufio.NewScanner(os.Stdin)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return "", err
		}
		return models[defaultIndex].ID, nil
	}

	answer := strings.TrimSpace(scanner.Text())
	if answer == "" {
		return models[defaultIndex].ID, nil
	}

	index, err := strconv.Atoi(answer)
	if err != nil || index < 1 || index > len(models) {
		return "", fmt.Errorf("invalid model selection %q", answer)
	}
	return models[index-1].ID, nil
}

func prepareLaunchExecution(target launchTarget, serverURL, modelID string, userArgs []string) (preparedLaunch, error) {
	switch target.AppID {
	case "claude-code":
		return prepareClaudeLaunch(target, serverURL, modelID, userArgs)
	case "open-code":
		return prepareOpenCodeLaunch(target, serverURL, modelID, userArgs)
	case "codex":
		return prepareCodexLaunch(target, serverURL, modelID, userArgs)
	case "openclaw":
		return prepareOpenClawLaunch(target, serverURL, modelID, userArgs)
	case "csgclaw":
		return prepareCSGClawLaunch(target, serverURL, modelID, userArgs)
	default:
		return preparedLaunch{}, fmt.Errorf("%s does not support direct launch yet", target.DisplayName)
	}
}

func prepareClaudeLaunch(target launchTarget, serverURL, modelID string, userArgs []string) (preparedLaunch, error) {
	binary, err := resolveLaunchBinary(target.Binaries)
	if err != nil {
		return preparedLaunch{}, fmt.Errorf("%s is installed, but the launch command was not found on PATH", target.DisplayName)
	}

	args := append([]string{}, userArgs...)
	args = prependArgsIfMissing(args, []string{"--model", modelID}, "--model", "-m")
	args = prependArgsIfMissing(args, []string{"--settings", claudeLaunchSettingsJSON(serverURL)}, "--settings")
	env := envWithOverrides(map[string]string{
		"ANTHROPIC_BASE_URL":   serverURL,
		"ANTHROPIC_AUTH_TOKEN": "csghub-lite",
		"ANTHROPIC_API_KEY":    "csghub-lite",
		"CLAUDE_API_BASE_URL":  serverURL,
		"CLAUDE_API_KEY":       "csghub-lite",
	})
	return preparedLaunch{Binary: binary, Args: args, Env: env}, nil
}

func prepareOpenCodeLaunch(target launchTarget, serverURL, modelID string, userArgs []string) (preparedLaunch, error) {
	binary, err := resolveLaunchBinary(target.Binaries)
	if err != nil {
		return preparedLaunch{}, fmt.Errorf("%s is installed, but the launch command was not found on PATH", target.DisplayName)
	}

	configPath, err := writeOpenCodeLaunchConfig(serverURL, modelID)
	if err != nil {
		return preparedLaunch{}, err
	}

	env := envWithOverrides(map[string]string{
		"OPENCODE_CONFIG": configPath,
	})
	return preparedLaunch{Binary: binary, Args: append([]string{}, userArgs...), Env: env}, nil
}

func prepareCodexLaunch(target launchTarget, serverURL, modelID string, userArgs []string) (preparedLaunch, error) {
	binary, err := resolveLaunchBinary(target.Binaries)
	if err != nil {
		return preparedLaunch{}, fmt.Errorf("%s is installed, but the launch command was not found on PATH", target.DisplayName)
	}
	models, err := getLaunchModels(serverURL)
	if err != nil {
		return preparedLaunch{}, err
	}

	args := append([]string{}, userArgs...)
	args = prependArgsIfMissing(args, []string{"--model", modelID}, "--model", "-m")
	args = prependDefaultCodexProviderConfig(args, serverURL)
	args, err = prependCodexModelCatalogConfig(args, models)
	if err != nil {
		return preparedLaunch{}, err
	}

	env := envWithOverrides(nil)
	return preparedLaunch{Binary: binary, Args: args, Env: env}, nil
}

func prepareOpenClawLaunch(target launchTarget, serverURL, modelID string, userArgs []string) (preparedLaunch, error) {
	binary, err := resolveLaunchBinary(target.Binaries)
	if err != nil {
		return preparedLaunch{}, fmt.Errorf("%s is installed, but the launch command was not found on PATH", target.DisplayName)
	}

	if err := ensureOpenClawProfile(binary, serverURL, modelID); err != nil {
		return preparedLaunch{}, err
	}

	args := prependArgsIfMissing(userArgs, []string{"--profile", openClawLaunchProfile}, "--profile")
	env := envWithOverrides(nil)
	return preparedLaunch{Binary: binary, Args: args, Env: env}, nil
}

func prepareCSGClawLaunch(target launchTarget, serverURL, modelID string, userArgs []string) (preparedLaunch, error) {
	binary, err := resolveLaunchBinary(target.Binaries)
	if err != nil {
		return preparedLaunch{}, fmt.Errorf("%s is installed, but the launch command was not found on PATH", target.DisplayName)
	}

	models, err := getLaunchModels(serverURL)
	if err != nil {
		return preparedLaunch{}, err
	}
	modelIDs := csgClawOrderedModels(modelID, launchModelIDs(models))
	modelBaseURL := strings.TrimRight(serverURL, "/") + "/v1"
	apiKey := openClawProviderAPIKey(config.Get().Token)

	ctx, cancel := context.WithTimeout(context.Background(), csgClawConfigureTimeout)
	defer cancel()

	onboardArgs := []string{
		"onboard",
		"--provider", csgClawLaunchProviderID,
		"--manager-image", csgClawManagerImage,
	}
	if csgClawLaunchNeedsManagerRecreate(modelBaseURL, apiKey, modelID, csgClawManagerImage) {
		onboardArgs = append(onboardArgs, "--force-recreate-manager")
	}
	onboardArgs = append(onboardArgs,
		"--base-url", modelBaseURL,
		"--api-key", apiKey,
		"--models", strings.Join(modelIDs, ","),
	)
	cmd := exec.CommandContext(ctx, binary, onboardArgs...)
	output, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return preparedLaunch{}, fmt.Errorf("configuring CSGClaw timed out after %s", csgClawConfigureTimeout)
	}
	if err != nil {
		msg := strings.TrimSpace(string(output))
		if msg == "" {
			msg = err.Error()
		}
		return preparedLaunch{}, fmt.Errorf("configuring CSGClaw: %s", msg)
	}

	args := append([]string{}, userArgs...)
	if len(args) == 0 {
		args = []string{"serve"}
	}
	return preparedLaunch{Binary: binary, Args: args, Env: envWithOverrides(nil)}, nil
}

func prependArgsIfMissing(args []string, defaults []string, flags ...string) []string {
	if hasAnyFlag(args, flags...) {
		return append([]string{}, args...)
	}
	merged := make([]string, 0, len(defaults)+len(args))
	merged = append(merged, defaults...)
	merged = append(merged, args...)
	return merged
}

func hasAnyFlag(args []string, flags ...string) bool {
	for _, arg := range args {
		for _, flag := range flags {
			if arg == flag || strings.HasPrefix(arg, flag+"=") {
				return true
			}
		}
	}
	return false
}

func prependCodexConfigIfMissing(args []string, key, value string) []string {
	target := key + "="
	for i := 0; i < len(args); i++ {
		if args[i] != "-c" && args[i] != "--config" {
			continue
		}
		if i+1 < len(args) && strings.HasPrefix(args[i+1], target) {
			return append([]string{}, args...)
		}
	}

	defaults := []string{"-c", fmt.Sprintf("%s=%s", key, value)}
	merged := make([]string, 0, len(defaults)+len(args))
	merged = append(merged, defaults...)
	merged = append(merged, args...)
	return merged
}

const codexLaunchProviderID = "csghub_lite"

func prependDefaultCodexProviderConfig(args []string, serverURL string) []string {
	baseURL := strings.TrimRight(serverURL, "/") + "/v1"
	defaults := [][2]string{
		{"model_provider", fmt.Sprintf("%q", codexLaunchProviderID)},
		{fmt.Sprintf("model_providers.%s.name", codexLaunchProviderID), `"OpenCSG"`},
		{fmt.Sprintf("model_providers.%s.base_url", codexLaunchProviderID), fmt.Sprintf("%q", baseURL)},
		{fmt.Sprintf("model_providers.%s.supports_websockets", codexLaunchProviderID), "false"},
	}
	for i := len(defaults) - 1; i >= 0; i-- {
		args = prependCodexConfigIfMissing(args, defaults[i][0], defaults[i][1])
	}
	return args
}

func prependCodexModelCatalogConfig(args []string, models []api.ModelInfo) ([]string, error) {
	path, err := writeCodexLaunchModelCatalog(models)
	if err != nil {
		return nil, err
	}
	return prependCodexConfigIfMissing(args, "model_catalog_json", fmt.Sprintf("%q", path)), nil
}

func envWithOverrides(overrides map[string]string) []string {
	env := append([]string{}, os.Environ()...)
	for key, value := range overrides {
		prefix := key + "="
		replaced := false
		for i, item := range env {
			if strings.HasPrefix(item, prefix) {
				env[i] = prefix + value
				replaced = true
				break
			}
		}
		if !replaced {
			env = append(env, prefix+value)
		}
	}
	return env
}

func claudeLaunchSettingsJSON(serverURL string) string {
	payload := map[string]interface{}{
		"env": map[string]string{
			"ANTHROPIC_BASE_URL":   serverURL,
			"ANTHROPIC_AUTH_TOKEN": "csghub-lite",
			"ANTHROPIC_API_KEY":    "csghub-lite",
			"CLAUDE_API_BASE_URL":  serverURL,
			"CLAUDE_API_KEY":       "csghub-lite",
		},
		"permissions": map[string]string{
			"defaultMode": "acceptEdits",
		},
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return `{"env":{}}`
	}
	return string(data)
}

func writeOpenCodeLaunchConfig(serverURL, modelID string) (string, error) {
	dir, err := launchDataDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("creating OpenCode launch config dir: %w", err)
	}

	payload := map[string]interface{}{
		"$schema":           "https://opencode.ai/config.json",
		"enabled_providers": []string{openCodeLaunchProviderID},
		"provider": map[string]interface{}{
			openCodeLaunchProviderID: map[string]interface{}{
				"npm":  "@ai-sdk/openai-compatible",
				"name": "OpenCSG",
				"options": map[string]interface{}{
					"baseURL": strings.TrimRight(serverURL, "/") + "/v1",
				},
				"models": map[string]interface{}{
					modelID: map[string]interface{}{
						"name": modelID,
					},
				},
			},
		},
		"model":       openCodeLaunchProviderID + "/" + modelID,
		"small_model": openCodeLaunchProviderID + "/" + modelID,
	}

	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return "", fmt.Errorf("encoding OpenCode launch config: %w", err)
	}

	path := filepath.Join(dir, "opencode.json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", fmt.Errorf("writing OpenCode launch config: %w", err)
	}
	return path, nil
}

func writeCodexLaunchModelCatalog(models []api.ModelInfo) (string, error) {
	dir, err := launchDataDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("creating Codex launch config dir: %w", err)
	}

	catalog := codexModelCatalog{
		Models: codexModelCatalogEntries(models),
	}
	if len(catalog.Models) == 0 {
		return "", fmt.Errorf("building Codex model catalog: no models available")
	}

	data, err := json.MarshalIndent(catalog, "", "  ")
	if err != nil {
		return "", fmt.Errorf("encoding Codex model catalog: %w", err)
	}

	path := filepath.Join(dir, "codex-models.json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", fmt.Errorf("writing Codex model catalog: %w", err)
	}
	return path, nil
}

func codexModelCatalogEntries(models []api.ModelInfo) []codexModelCatalogEntry {
	entries := make([]codexModelCatalogEntry, 0, len(models))
	seen := make(map[string]struct{}, len(models))
	for _, item := range models {
		modelID := strings.TrimSpace(item.Model)
		if modelID == "" {
			continue
		}
		if _, ok := seen[modelID]; ok {
			continue
		}
		seen[modelID] = struct{}{}

		displayName := strings.TrimSpace(item.DisplayName)
		if displayName == "" {
			displayName = modelID
		}
		description := "Model served by OpenCSG."
		switch strings.TrimSpace(item.Source) {
		case "local":
			description = "Local model served by OpenCSG."
		case "cloud":
			description = "Cloud model served by OpenCSG."
		}

		inputModalities := []string{"text"}
		if item.HasMMProj || strings.EqualFold(strings.TrimSpace(item.PipelineTag), "image-text-to-text") {
			inputModalities = append(inputModalities, "image")
		}

		entries = append(entries, codexModelCatalogEntry{
			Slug:                       modelID,
			DisplayName:                displayName,
			Description:                description,
			SupportedReasoningLevels:   []codexReasoningEffortPreset{},
			ShellType:                  "shell_command",
			Visibility:                 "list",
			SupportedInAPI:             true,
			Priority:                   len(entries),
			BaseInstructions:           codexBaseInstructions,
			SupportsReasoningSummaries: false,
			SupportVerbosity:           false,
			TruncationPolicy: codexTruncationPolicy{
				Mode:  "bytes",
				Limit: 10_000,
			},
			SupportsParallelToolCalls:  false,
			ExperimentalSupportedTools: []string{},
			InputModalities:            inputModalities,
			ContextWindow:              codexContextWindowForModel(item),
		})
	}
	return entries
}

func codexContextWindowForModel(item api.ModelInfo) int64 {
	if item.ContextWindow > 0 {
		return item.ContextWindow
	}
	if strings.EqualFold(strings.TrimSpace(item.Source), "local") {
		return codexLocalContextWindow
	}
	return codexCloudContextWindow
}

func launchDataDir() (string, error) {
	appHome, err := config.AppHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(appHome, "apps", "launch"), nil
}

func ensureOpenClawProfile(binary, serverURL, modelID string) error {
	ok, err := openClawProfileMatches(serverURL, modelID)
	if err != nil || !ok {
		ctx, cancel := context.WithTimeout(context.Background(), openClawConfigureTimeout)
		defer cancel()

		args := []string{
			"--profile", openClawLaunchProfile,
			"onboard",
			"--non-interactive",
			"--auth-choice", "custom-api-key",
			"--custom-provider-id", openClawLaunchProviderID,
			"--custom-compatibility", "openai",
			"--custom-base-url", openClawProviderBaseURL(serverURL),
			"--custom-model-id", modelID,
			"--custom-api-key", openClawProviderAPIKey(config.Get().Token),
			"--accept-risk",
			"--skip-channels",
			"--skip-search",
			"--skip-ui",
			"--skip-skills",
			"--skip-daemon",
			"--skip-health",
		}

		cmd := exec.CommandContext(ctx, binary, args...)
		cmd.Env = envWithOverrides(map[string]string{
			"NPM_CONFIG_REGISTRY": openClawNPMRegistry(),
		})
		output, err := cmd.CombinedOutput()
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("configuring OpenClaw timed out after %s", openClawConfigureTimeout)
		}
		if err != nil {
			msg := strings.TrimSpace(string(output))
			if msg == "" {
				msg = err.Error()
			}
			return fmt.Errorf("configuring OpenClaw: %s", msg)
		}
	}

	availableModels, err := getLaunchModels(serverURL)
	if err != nil {
		return err
	}
	modelIDs := make([]string, 0, len(availableModels))
	for _, item := range availableModels {
		modelID := strings.TrimSpace(item.Model)
		if modelID != "" {
			modelIDs = append(modelIDs, modelID)
		}
	}

	models := buildOpenClawProfileModels(modelIDs, availableModels)
	if err := syncOpenClawProfile(serverURL, openClawProviderAPIKey(config.Get().Token), modelID, models); err != nil {
		return fmt.Errorf("syncing OpenClaw profile models: %w", err)
	}
	return nil
}

func openClawNPMRegistry() string {
	if registry := strings.TrimSpace(os.Getenv("NPM_CONFIG_REGISTRY")); registry != "" {
		return registry
	}
	return openClawDefaultRegistry
}

func openClawProviderBaseURL(serverURL string) string {
	return strings.TrimRight(serverURL, "/") + "/v1"
}

func openClawProfileMatches(serverURL, modelID string) (bool, error) {
	path, err := openClawProfileConfigPath()
	if err != nil {
		return false, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}

	var cfg struct {
		Models struct {
			Providers map[string]struct {
				BaseURL string `json:"baseUrl"`
			} `json:"providers"`
		} `json:"models"`
		Agents struct {
			Defaults struct {
				Model struct {
					Primary string `json:"primary"`
				} `json:"model"`
			} `json:"defaults"`
		} `json:"agents"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return false, err
	}

	provider, ok := cfg.Models.Providers[openClawLaunchProviderID]
	if !ok {
		return false, nil
	}
	wantModel := openClawLaunchProviderID + "/" + modelID
	return strings.TrimRight(provider.BaseURL, "/") == strings.TrimRight(openClawProviderBaseURL(serverURL), "/") &&
		cfg.Agents.Defaults.Model.Primary == wantModel, nil
}

func openClawProfileConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	base := ".openclaw-" + openClawLaunchProfile
	if openClawLaunchProfile == "" {
		base = ".openclaw"
	}
	return filepath.Join(home, base, "openclaw.json"), nil
}

func openClawAgentModelsPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	base := ".openclaw-" + openClawLaunchProfile
	if openClawLaunchProfile == "" {
		base = ".openclaw"
	}
	return filepath.Join(home, base, "agents", "main", "agent", "models.json"), nil
}

func buildOpenClawProfileModels(modelIDs []string, available []api.ModelInfo) []api.ModelInfo {
	byID := make(map[string]api.ModelInfo, len(available))
	for _, item := range available {
		modelID := strings.TrimSpace(item.Model)
		if modelID == "" {
			continue
		}
		byID[modelID] = item
	}

	models := make([]api.ModelInfo, 0, len(modelIDs))
	seen := make(map[string]struct{}, len(modelIDs))
	for _, modelID := range modelIDs {
		modelID = strings.TrimSpace(modelID)
		if modelID == "" {
			continue
		}
		if _, ok := seen[modelID]; ok {
			continue
		}
		seen[modelID] = struct{}{}
		if item, ok := byID[modelID]; ok {
			models = append(models, item)
			continue
		}
		models = append(models, api.ModelInfo{
			Name:        modelID,
			Model:       modelID,
			DisplayName: modelID,
		})
	}
	return models
}

func syncOpenClawProfile(serverURL, apiKey, selectedModelID string, models []api.ModelInfo) error {
	provider := openClawProviderConfig(serverURL, apiKey, models)
	primaryModel := openClawLaunchProviderID + "/" + strings.TrimSpace(selectedModelID)
	agentModels := openClawAgentModelEntries(models)

	profilePath, err := openClawProfileConfigPath()
	if err != nil {
		return err
	}
	if err := syncOpenClawJSONFile(profilePath, func(doc map[string]interface{}) {
		modelsSection := ensureOpenClawObject(doc, "models")
		if strings.TrimSpace(fmt.Sprint(modelsSection["mode"])) == "" {
			modelsSection["mode"] = "merge"
		}
		modelsSection["providers"] = map[string]interface{}{
			openClawLaunchProviderID: provider,
		}

		agentsSection := ensureOpenClawObject(doc, "agents")
		defaultsSection := ensureOpenClawObject(agentsSection, "defaults")
		modelSection := ensureOpenClawObject(defaultsSection, "model")
		modelSection["primary"] = primaryModel
		defaultsSection["models"] = agentModels
	}); err != nil {
		return err
	}

	modelsPath, err := openClawAgentModelsPath()
	if err != nil {
		return err
	}
	return syncOpenClawJSONFile(modelsPath, func(doc map[string]interface{}) {
		doc["providers"] = map[string]interface{}{
			openClawLaunchProviderID: provider,
		}
	})
}

func launchModelIDs(models []api.ModelInfo) []string {
	ids := make([]string, 0, len(models))
	for _, item := range models {
		modelID := strings.TrimSpace(item.Model)
		if modelID != "" {
			ids = append(ids, modelID)
		}
	}
	return ids
}

func csgClawOrderedModels(selected string, modelIDs []string) []string {
	selected = strings.TrimSpace(selected)
	ordered := make([]string, 0, len(modelIDs)+1)
	seen := make(map[string]struct{}, len(modelIDs)+1)
	appendModel := func(modelID string) {
		modelID = strings.TrimSpace(modelID)
		if modelID == "" {
			return
		}
		if _, ok := seen[modelID]; ok {
			return
		}
		seen[modelID] = struct{}{}
		ordered = append(ordered, modelID)
	}
	appendModel(selected)
	for _, modelID := range modelIDs {
		appendModel(modelID)
	}
	return ordered
}

func csgClawLaunchNeedsManagerRecreate(baseURL, apiKey, modelID, managerImage string) bool {
	path, err := csgClawConfigPath()
	if err != nil {
		return true
	}
	file, err := os.Open(path)
	if err != nil {
		return true
	}
	defer file.Close()

	defaultSelector := ""
	currentManagerImage := ""
	providerBaseURL := ""
	providerAPIKey := ""
	providerModels := []string{}
	section := ""
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.Trim(line, "[]")
			continue
		}
		key, value, ok := parseSimpleConfigKV(line)
		if !ok {
			continue
		}
		switch section {
		case "bootstrap":
			if key == "manager_image" {
				currentManagerImage = value
			}
		case "models":
			if key == "default" {
				defaultSelector = value
			}
		case "models.providers." + csgClawLaunchProviderID:
			switch key {
			case "base_url":
				providerBaseURL = value
			case "api_key":
				providerAPIKey = value
			case "models":
				providerModels = parseSimpleStringArray(value)
			}
		}
	}
	if scanner.Err() != nil {
		return true
	}

	wantSelector := csgClawLaunchProviderID + "." + strings.TrimSpace(modelID)
	return strings.TrimSpace(defaultSelector) != wantSelector ||
		strings.TrimSpace(currentManagerImage) != strings.TrimSpace(managerImage) ||
		strings.TrimRight(providerBaseURL, "/") != strings.TrimRight(baseURL, "/") ||
		strings.TrimSpace(providerAPIKey) != strings.TrimSpace(apiKey) ||
		!containsString(providerModels, modelID)
}

func csgClawConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".csgclaw", "config.toml"), nil
}

func parseSimpleConfigKV(line string) (key, value string, ok bool) {
	key, value, ok = strings.Cut(line, "=")
	if !ok {
		return "", "", false
	}
	key = strings.TrimSpace(key)
	value = strings.TrimSpace(value)
	if len(value) >= 2 && strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"") {
		if unquoted, err := strconv.Unquote(value); err == nil {
			value = unquoted
		}
	}
	return key, value, true
}

func parseSimpleStringArray(value string) []string {
	value = strings.TrimSpace(value)
	if !strings.HasPrefix(value, "[") || !strings.HasSuffix(value, "]") {
		return nil
	}
	inner := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(value, "["), "]"))
	if inner == "" {
		return nil
	}
	parts := strings.Split(inner, ",")
	items := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if len(part) >= 2 && strings.HasPrefix(part, "\"") && strings.HasSuffix(part, "\"") {
			if unquoted, err := strconv.Unquote(part); err == nil {
				part = unquoted
			}
		}
		if part != "" {
			items = append(items, part)
		}
	}
	return items
}

func containsString(items []string, want string) bool {
	want = strings.TrimSpace(want)
	for _, item := range items {
		if strings.TrimSpace(item) == want {
			return true
		}
	}
	return false
}

func syncOpenClawJSONFile(path string, mutate func(map[string]interface{})) error {
	doc := map[string]interface{}{}
	if data, err := os.ReadFile(path); err == nil {
		if len(bytes.TrimSpace(data)) > 0 {
			if err := json.Unmarshal(data, &doc); err != nil {
				return err
			}
		}
	} else if !os.IsNotExist(err) {
		return err
	}

	mutate(doc)

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func ensureOpenClawObject(parent map[string]interface{}, key string) map[string]interface{} {
	if existing, ok := parent[key].(map[string]interface{}); ok {
		return existing
	}
	child := map[string]interface{}{}
	parent[key] = child
	return child
}

func openClawProviderConfig(serverURL, apiKey string, models []api.ModelInfo) map[string]interface{} {
	return map[string]interface{}{
		"baseUrl": openClawProviderBaseURL(serverURL),
		"apiKey":  openClawProviderAPIKey(apiKey),
		"api":     openClawLaunchProviderAPI,
		"models":  openClawProviderModels(models),
	}
}

func openClawProviderAPIKey(token string) string {
	token = strings.TrimSpace(token)
	if token == "" {
		return "csghub-lite"
	}
	return token
}

func openClawProviderModels(models []api.ModelInfo) []map[string]interface{} {
	items := make([]map[string]interface{}, 0, len(models))
	for _, item := range models {
		modelID := strings.TrimSpace(item.Model)
		if modelID == "" {
			continue
		}

		displayName := strings.TrimSpace(item.DisplayName)
		if displayName == "" {
			displayName = modelID
		}
		source := strings.TrimSpace(item.Source)
		if source == "cloud" {
			displayName += " (OpenCSG)"
		} else if source == "local" {
			displayName += " (Local)"
		}

		items = append(items, map[string]interface{}{
			"id":            modelID,
			"name":          displayName,
			"api":           openClawLaunchProviderAPI,
			"reasoning":     false,
			"input":         []string{"text"},
			"cost":          openClawZeroCost(),
			"contextWindow": openClawContextWindow,
			"maxTokens":     openClawMaxTokens,
		})
	}
	return items
}

func openClawAgentModelEntries(models []api.ModelInfo) map[string]interface{} {
	entries := make(map[string]interface{}, len(models))
	for _, item := range models {
		modelID := strings.TrimSpace(item.Model)
		if modelID == "" {
			continue
		}
		entries[openClawLaunchProviderID+"/"+modelID] = map[string]interface{}{}
	}
	return entries
}

func openClawZeroCost() map[string]float64 {
	return map[string]float64{
		"input":      0,
		"output":     0,
		"cacheRead":  0,
		"cacheWrite": 0,
	}
}
