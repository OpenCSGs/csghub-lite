package cli

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"text/tabwriter"
	"time"

	"github.com/opencsgs/csghub-lite/internal/config"
	"github.com/opencsgs/csghub-lite/pkg/api"
	"github.com/spf13/cobra"
)

func newPsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ps",
		Short: "List currently running models",
		Long:  "List models that are currently loaded in the server and their resource usage.",
		RunE:  runPs,
	}
	return cmd
}

func runPs(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	serverURL := fmt.Sprintf("http://localhost%s", cfg.ListenAddr)
	client := &http.Client{Timeout: 5 * time.Second}

	resp, err := client.Get(serverURL + "/api/ps")
	if err != nil {
		return fmt.Errorf("cannot connect to csghub-lite server at %s. Is it running? Start it with 'csghub-lite serve'", serverURL)
	}
	defer resp.Body.Close()

	var psResp api.PsResponse
	if err := json.NewDecoder(resp.Body).Decode(&psResp); err != nil {
		return fmt.Errorf("decoding response: %w", err)
	}

	if len(psResp.Models) == 0 {
		fmt.Println("No models currently running.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "NAME\tFORMAT\tSIZE\tUNTIL")
	for _, m := range psResp.Models {
		until := "forever"
		if !m.ExpiresAt.IsZero() {
			until = time.Until(m.ExpiresAt).Truncate(time.Second).String()
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
			m.Name, m.Format, formatBytes(m.Size), until)
	}
	return w.Flush()
}
