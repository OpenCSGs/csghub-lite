package cli

import (
	"fmt"

	"github.com/opencsgs/csghub-lite/internal/config"
	"github.com/opencsgs/csghub-lite/internal/inference"
	"github.com/opencsgs/csghub-lite/internal/model"
	"github.com/spf13/cobra"
)

func newChatCmd() *cobra.Command {
	var systemPrompt string
	var verbose bool

	cmd := &cobra.Command{
		Use:   "chat MODEL",
		Short: "Start an interactive chat with a local model",
		Long: `Start an interactive chat session with a locally downloaded model.
Unlike 'run', this command does not auto-download missing models.

Type your message and press Enter to send. Use '/bye' to exit.
Multiline input: end a line with '\' to continue on the next line.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runChat(cmd, args, systemPrompt, verbose)
		},
	}

	cmd.Flags().StringVar(&systemPrompt, "system", "", "set a custom system prompt")
	cmd.Flags().BoolVar(&verbose, "verbose", false, "show detailed llama-server output")
	return cmd
}

func runChat(cmd *cobra.Command, args []string, systemPrompt string, verbose bool) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	mgr := model.NewManager(cfg)
	modelID := args[0]

	if !mgr.Exists(modelID) {
		return fmt.Errorf("model %q not found locally. Use 'csghub-lite pull %s' to download it first", modelID, modelID)
	}

	modelDir, err := mgr.ModelPath(modelID)
	if err != nil {
		return err
	}

	lm, err := mgr.Get(modelID)
	if err != nil {
		return err
	}

	fmt.Printf("Loading %s...\n", modelID)
	eng, err := inference.LoadEngineWithProgress(modelDir, lm, convertProgress, verbose)
	if err != nil {
		return fmt.Errorf("loading model: %w", err)
	}
	defer eng.Close()

	fmt.Printf("Model %s ready. Type '/bye' to exit, '/clear' to reset context, '/help' for more.\n\n", modelID)

	opts := inference.DefaultOptions()
	session := inference.NewSession(eng, opts)

	if systemPrompt != "" {
		session.SetSystemPrompt(systemPrompt)
	}

	return chatLoop(cmd.Context(), session)
}
