package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/opencsgs/csghub-lite/internal/config"
	"github.com/opencsgs/csghub-lite/internal/model"
)

const (
	openCodeLaunchProviderID = "csghub-lite"
	openClawLaunchProfile    = "csghub-lite"
	openClawLaunchProviderID = "csghub"
	openClawConfigureTimeout = 2 * time.Minute
)

type preparedLaunch struct {
	Binary string
	Args   []string
	Env    []string
}

func resolveLaunchModel(cfg *config.Config, requested string, skipPrompt bool) (string, error) {
	mgr := model.NewManager(cfg)
	models, err := mgr.List()
	if err != nil {
		return "", fmt.Errorf("listing local models: %w", err)
	}
	if len(models) == 0 {
		return "", fmt.Errorf("no local models were found. Use 'csghub-lite pull MODEL' before launching an AI app")
	}

	sort.SliceStable(models, func(i, j int) bool {
		left := scoreLaunchModel(models[i])
		right := scoreLaunchModel(models[j])
		if left != right {
			return left > right
		}
		return models[i].DownloadedAt.After(models[j].DownloadedAt)
	})

	if requested != "" {
		for _, candidate := range models {
			if candidate.FullName() == requested {
				return candidate.FullName(), nil
			}
		}
		return "", fmt.Errorf("model %q is not downloaded locally. Use 'csghub-lite list' to see local models", requested)
	}

	if len(models) == 1 || skipPrompt || !stdinIsTerminal() {
		return models[0].FullName(), nil
	}

	return promptForLaunchModel(models)
}

func scoreLaunchModel(m *model.LocalModel) int64 {
	name := strings.ToLower(m.FullName())
	score := m.Size / 1_000_000
	if strings.Contains(name, "coder") {
		score += 10_000_000
	}
	if strings.Contains(name, "code") {
		score += 5_000_000
	}
	if strings.Contains(name, "gpt-oss") {
		score += 6_000_000
	}
	if strings.Contains(name, "qwen") {
		score += 2_000_000
	}
	return score
}

func stdinIsTerminal() bool {
	info, err := os.Stdin.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

func promptForLaunchModel(models []*model.LocalModel) (string, error) {
	fmt.Fprintln(os.Stderr, "Select a local model for AI apps:")
	for i, candidate := range models {
		label := candidate.FullName()
		if i == 0 {
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
		return models[0].FullName(), nil
	}

	answer := strings.TrimSpace(scanner.Text())
	if answer == "" {
		return models[0].FullName(), nil
	}

	index, err := strconv.Atoi(answer)
	if err != nil || index < 1 || index > len(models) {
		return "", fmt.Errorf("invalid model selection %q", answer)
	}
	return models[index-1].FullName(), nil
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

	args := append([]string{}, userArgs...)
	args = prependArgsIfMissing(args, []string{"--model", modelID}, "--model", "-m")
	args = prependCodexConfigIfMissing(args, "openai_base_url", strings.TrimRight(serverURL, "/")+"/v1")

	env := envWithOverrides(map[string]string{
		"OPENAI_API_KEY": "csghub-lite",
	})
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

	defaults := []string{"-c", fmt.Sprintf("%s=%q", key, value)}
	merged := make([]string, 0, len(defaults)+len(args))
	merged = append(merged, defaults...)
	merged = append(merged, args...)
	return merged
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
				"name": "CSGHub Lite",
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

func launchDataDir() (string, error) {
	appHome, err := config.AppHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(appHome, "apps", "launch"), nil
}

func ensureOpenClawProfile(binary, serverURL, modelID string) error {
	ok, err := openClawProfileMatches(serverURL, modelID)
	if err == nil && ok {
		return nil
	}

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
		"--custom-api-key", "csghub-lite",
		"--accept-risk",
		"--skip-channels",
		"--skip-search",
		"--skip-ui",
		"--skip-skills",
		"--skip-daemon",
		"--skip-health",
	}

	cmd := exec.CommandContext(ctx, binary, args...)
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
	return nil
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
