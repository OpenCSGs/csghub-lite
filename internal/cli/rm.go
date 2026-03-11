package cli

import (
	"fmt"

	"github.com/opencsgs/csghub-lite/internal/config"
	"github.com/opencsgs/csghub-lite/internal/model"
	"github.com/spf13/cobra"
)

func newRmCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rm MODEL",
		Short: "Remove a locally downloaded model",
		Args:  cobra.ExactArgs(1),
		RunE:  runRm,
	}
	return cmd
}

func runRm(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	mgr := model.NewManager(cfg)
	modelID := args[0]

	if err := mgr.Remove(modelID); err != nil {
		return fmt.Errorf("removing model: %w", err)
	}

	fmt.Printf("Removed %s\n", modelID)
	return nil
}
