package cli

import (
	"fmt"

	"github.com/opencsgs/csghub-lite/internal/config"
	"github.com/opencsgs/csghub-lite/internal/csghub"
	"github.com/opencsgs/csghub-lite/internal/model"
	"github.com/spf13/cobra"
)

func newPullCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pull MODEL",
		Short: "Download a model from CSGHub",
		Long:  "Download a model from the CSGHub platform. MODEL should be in the format namespace/name.",
		Args:  cobra.ExactArgs(1),
		RunE:  runPull,
	}
	return cmd
}

func runPull(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	mgr := model.NewManager(cfg)
	modelID := args[0]

	fmt.Printf("Pulling %s from %s...\n", modelID, cfg.ServerURL)

	var lastFile string
	progress := func(p csghub.SnapshotProgress) {
		if p.FileName != lastFile {
			if lastFile != "" {
				fmt.Println()
			}
			lastFile = p.FileName
			fmt.Printf("  [%d/%d] %s", p.FileIndex+1, p.TotalFiles, p.FileName)
		}
		if p.BytesTotal > 0 {
			pct := float64(p.BytesCompleted) / float64(p.BytesTotal) * 100
			fmt.Printf("\r  [%d/%d] %s  %.1f%% (%s / %s)",
				p.FileIndex+1, p.TotalFiles, p.FileName,
				pct, formatBytes(p.BytesCompleted), formatBytes(p.BytesTotal))
		}
	}

	lm, err := mgr.Pull(cmd.Context(), modelID, progress)
	if err != nil {
		return fmt.Errorf("pull failed: %w", err)
	}

	fmt.Printf("\n\nSuccessfully pulled %s (%s, %s)\n",
		lm.FullName(), lm.Format, formatBytes(lm.Size))
	return nil
}

func formatBytes(b int64) string {
	const (
		kb = 1024
		mb = kb * 1024
		gb = mb * 1024
	)
	switch {
	case b >= gb:
		return fmt.Sprintf("%.1f GB", float64(b)/float64(gb))
	case b >= mb:
		return fmt.Sprintf("%.1f MB", float64(b)/float64(mb))
	case b >= kb:
		return fmt.Sprintf("%.1f KB", float64(b)/float64(kb))
	default:
		return fmt.Sprintf("%d B", b)
	}
}
