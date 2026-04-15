package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/opencsgs/csghub-lite/internal/config"
	"github.com/spf13/cobra"
)

const supportedConfigKeys = "server_url, ai_gateway_url, storage_dir, model_dir, dataset_dir, listen_addr, token"

func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage csghub-lite configuration",
	}

	cmd.AddCommand(newConfigSetCmd(), newConfigGetCmd(), newConfigShowCmd())
	return cmd
}

func newConfigSetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set KEY VALUE",
		Short: "Set a configuration value",
		Long: `Set a configuration value.

Available keys:
  server_url       CSGHub server URL for model marketplace (default: https://hub.opencsg.com)
  ai_gateway_url   AI Gateway URL for cloud inference models (default: https://ai.space.opencsg.com)
  storage_dir      Root storage directory (sets both model_dir and dataset_dir)
  model_dir        Directory for downloaded models
  dataset_dir      Directory for downloaded datasets
  listen_addr      Local server listen address (default: :11435)
  token            Access token for CSGHub authentication

Examples:
  csghub-lite config set server_url https://my-csghub.example.com
  csghub-lite config set ai_gateway_url https://my-gateway.example.com
  csghub-lite config set storage_dir /data/csghub-lite
  csghub-lite config set listen_addr :8080`,
		Args: cobra.ExactArgs(2),
		RunE: runConfigSet,
	}
}

func newConfigGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get KEY",
		Short: "Get a configuration value",
		Long: `Get a configuration value.

Available keys: ` + supportedConfigKeys + `

Examples:
  csghub-lite config get server_url
  csghub-lite config get ai_gateway_url`,
		Args: cobra.ExactArgs(1),
		RunE: runConfigGet,
	}
}

func newConfigShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Show all configuration",
		RunE:  runConfigShow,
	}
}

func runConfigSet(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	key, value := strings.TrimSpace(args[0]), args[1]
	switch key {
	case "server_url":
		cfg.ServerURL = strings.TrimSpace(value)
	case "ai_gateway_url":
		cfg.AIGatewayURL = strings.TrimSpace(value)
	case "storage_dir":
		dir, err := requiredPathValue(value)
		if err != nil {
			return fmt.Errorf("invalid storage_dir: %w", err)
		}
		cfg.ModelDir = config.ModelDirForStorage(dir)
		cfg.DatasetDir = config.DatasetDirForStorage(dir)
		if err := ensureDir(cfg.ModelDir); err != nil {
			return fmt.Errorf("creating model directory: %w", err)
		}
		if err := ensureDir(cfg.DatasetDir); err != nil {
			return fmt.Errorf("creating dataset directory: %w", err)
		}
	case "model_dir":
		dir, err := requiredPathValue(value)
		if err != nil {
			return fmt.Errorf("invalid model_dir: %w", err)
		}
		cfg.ModelDir = dir
		if err := ensureDir(cfg.ModelDir); err != nil {
			return fmt.Errorf("creating model directory: %w", err)
		}
	case "dataset_dir":
		dir, err := requiredPathValue(value)
		if err != nil {
			return fmt.Errorf("invalid dataset_dir: %w", err)
		}
		cfg.DatasetDir = dir
		if err := ensureDir(cfg.DatasetDir); err != nil {
			return fmt.Errorf("creating dataset directory: %w", err)
		}
	case "listen_addr":
		cfg.ListenAddr = strings.TrimSpace(value)
	case "token":
		cfg.Token = strings.TrimSpace(value)
	default:
		return fmt.Errorf("unknown config key %q (valid: %s)", key, supportedConfigKeys)
	}

	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}

	fmt.Printf("Set %s = %s\n", key, displayConfigValue(cfg, key))
	return nil
}

func runConfigGet(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	key := args[0]
	switch key {
	case "server_url":
		fmt.Println(cfg.ServerURL)
	case "ai_gateway_url":
		fmt.Println(cfg.AIGatewayURL)
	case "storage_dir":
		fmt.Println(cfg.StorageDir())
	case "model_dir":
		fmt.Println(cfg.ModelDir)
	case "dataset_dir":
		fmt.Println(cfg.DatasetDir)
	case "listen_addr":
		fmt.Println(cfg.ListenAddr)
	case "token":
		fmt.Println(maskedToken(cfg.Token))
	default:
		return fmt.Errorf("unknown config key %q (valid: %s)", key, supportedConfigKeys)
	}
	return nil
}

func runConfigShow(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	fmt.Printf("server_url:      %s\n", cfg.ServerURL)
	fmt.Printf("ai_gateway_url:  %s\n", cfg.AIGatewayURL)
	fmt.Printf("storage_dir:     %s\n", cfg.StorageDir())
	fmt.Printf("model_dir:       %s\n", cfg.ModelDir)
	fmt.Printf("dataset_dir:     %s\n", cfg.DatasetDir)
	fmt.Printf("listen_addr:     %s\n", cfg.ListenAddr)
	fmt.Printf("token:           %s\n", maskedToken(cfg.Token))
	return nil
}

func requiredPathValue(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", fmt.Errorf("path cannot be empty")
	}
	return filepath.Clean(trimmed), nil
}

func ensureDir(path string) error {
	return os.MkdirAll(path, 0o755)
}

func maskedToken(token string) string {
	if token == "" {
		return "(not set)"
	}
	if len(token) <= 4 {
		return token + "****"
	}
	return token[:4] + "****"
}

func displayConfigValue(cfg *config.Config, key string) string {
	switch key {
	case "storage_dir":
		return cfg.StorageDir()
	case "model_dir":
		return cfg.ModelDir
	case "dataset_dir":
		return cfg.DatasetDir
	case "listen_addr":
		return cfg.ListenAddr
	case "server_url":
		return cfg.ServerURL
	case "ai_gateway_url":
		return cfg.AIGatewayURL
	case "token":
		return maskedToken(cfg.Token)
	default:
		return ""
	}
}
