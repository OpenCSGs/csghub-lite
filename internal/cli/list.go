package cli

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/opencsgs/csghub-lite/internal/config"
	"github.com/opencsgs/csghub-lite/internal/model"
	"github.com/spf13/cobra"
)

func newListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List locally available models",
		RunE:    runList,
	}
	return cmd
}

func runList(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	mgr := model.NewManager(cfg)
	models, err := mgr.List()
	if err != nil {
		return fmt.Errorf("listing models: %w", err)
	}

	if len(models) == 0 {
		fmt.Println("No models downloaded. Use 'csghub-lite pull' to download a model.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "NAME\tFORMAT\tSIZE\tDOWNLOADED")
	for _, m := range models {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
			m.FullName(),
			m.Format,
			formatBytes(m.Size),
			m.DownloadedAt.Format("2006-01-02 15:04"),
		)
	}
	return w.Flush()
}
