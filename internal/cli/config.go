package cli

import (
	"fmt"

	"github.com/opencsgs/csghub-lite/internal/config"
	"github.com/spf13/cobra"
)

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
		Short: "Set a configuration value (server_url, model_dir, listen_addr)",
		Args:  cobra.ExactArgs(2),
		RunE:  runConfigSet,
	}
}

func newConfigGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get KEY",
		Short: "Get a configuration value",
		Args:  cobra.ExactArgs(1),
		RunE:  runConfigGet,
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

	key, value := args[0], args[1]
	switch key {
	case "server_url":
		cfg.ServerURL = value
	case "model_dir":
		cfg.ModelDir = value
	case "listen_addr":
		cfg.ListenAddr = value
	default:
		return fmt.Errorf("unknown config key %q (valid: server_url, model_dir, listen_addr)", key)
	}

	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}

	fmt.Printf("Set %s = %s\n", key, value)
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
	case "model_dir":
		fmt.Println(cfg.ModelDir)
	case "listen_addr":
		fmt.Println(cfg.ListenAddr)
	case "token":
		if cfg.Token != "" {
			fmt.Println(cfg.Token[:4] + "****")
		} else {
			fmt.Println("(not set)")
		}
	default:
		return fmt.Errorf("unknown config key %q", key)
	}
	return nil
}

func runConfigShow(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	fmt.Printf("server_url:  %s\n", cfg.ServerURL)
	fmt.Printf("model_dir:   %s\n", cfg.ModelDir)
	fmt.Printf("listen_addr: %s\n", cfg.ListenAddr)
	if cfg.Token != "" {
		fmt.Printf("token:       %s****\n", cfg.Token[:4])
	} else {
		fmt.Println("token:       (not set)")
	}
	return nil
}
