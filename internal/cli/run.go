package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/opencsgs/csghub-lite/internal/config"
	"github.com/opencsgs/csghub-lite/internal/convert"
	"github.com/opencsgs/csghub-lite/internal/inference"
	"github.com/opencsgs/csghub-lite/internal/model"
	"github.com/spf13/cobra"
)

func newRunCmd() *cobra.Command {
	var numCtx int
	var numParallel int

	cmd := &cobra.Command{
		Use:   "run MODEL",
		Short: "Download (if needed) and chat with a model",
		Long: `Download a model from CSGHub if not already present, then start an interactive
chat session. Type your message and press Enter to send. Use '/bye' to exit.

Multiline input: end a line with '\' to continue on the next line.

Use --num-ctx and --num-parallel to override llama-server context settings for
this run only.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRun(cmd, args, numCtx, numParallel)
		},
	}
	cmd.Flags().IntVar(&numCtx, "num-ctx", 0, "set the per-model context length for this run only (for example 131072)")
	cmd.Flags().IntVar(&numParallel, "num-parallel", 0, "set the llama-server parallel slots for this run only (use 1 to maximize single-session context)")
	return cmd
}

func validateInteractiveModelOverrides(numCtx, numParallel int) error {
	if numCtx > 0 && numCtx < 1024 {
		return fmt.Errorf("--num-ctx must be at least 1024 when set")
	}
	if numParallel < 0 {
		return fmt.Errorf("--num-parallel must be at least 1 when set")
	}
	return nil
}

func runRun(cmd *cobra.Command, args []string, numCtx, numParallel int) error {
	if err := validateInteractiveModelOverrides(numCtx, numParallel); err != nil {
		return err
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	mgr := model.NewManager(cfg)
	modelID := args[0]

	// Pull model if not present
	if !mgr.Exists(modelID) {
		fmt.Printf("Model %s not found locally. Pulling from %s...\n", modelID, cfg.DisplayURL())
		if _, err := mgr.Pull(cmd.Context(), modelID, snapshotProgress()); err != nil {
			return fmt.Errorf("pull failed: %w", err)
		}
		fmt.Println("Pull complete.")
	}

	fmt.Printf("Loading %s...\n", modelID)

	if modelDir, err := mgr.ModelPath(modelID); err == nil && convert.NeedsConversion(modelDir) {
		fmt.Println("Converting model to GGUF format (first time only, this may take a moment)...")
	}

	serverURL, err := ensureServer(cfg)
	if err != nil {
		return fmt.Errorf("starting server: %w", err)
	}

	if err := preloadModel(serverURL, modelID, numCtx, numParallel); err != nil {
		return fmt.Errorf("loading model: %w", err)
	}

	eng := inference.NewRemoteEngine(serverURL, modelID, numCtx, numParallel)

	fmt.Printf("Model %s ready. Type '/bye' to exit, '/clear' to reset context.\n\n", modelID)

	opts := inference.DefaultOptions()
	session := inference.NewSession(eng, opts)

	return chatLoop(cmd.Context(), session)
}

func chatLoop(ctx context.Context, session *Session) error {
	scanner := bufio.NewScanner(os.Stdin)

	for {
		fmt.Print(">>> ")
		input, ok := readMultilineInput(scanner)
		if !ok {
			fmt.Println()
			return nil
		}

		input = strings.TrimSpace(input)
		if input == "" {
			continue
		}

		switch strings.ToLower(input) {
		case "/bye", "/exit", "/quit":
			fmt.Println("Goodbye!")
			return nil
		case "/clear":
			session = inference.NewSession(session.Engine(), session.Options())
			fmt.Println("Context cleared.")
			continue
		case "/help":
			printHelp()
			continue
		}

		onToken := func(token string) {
			fmt.Print(token)
		}

		_, err := session.Send(ctx, input, onToken)
		if err != nil {
			fmt.Printf("\nError: %v\n", err)
			continue
		}
		fmt.Println()
		fmt.Println()
	}
}

// Session wraps inference.Session for the chat loop, providing mutable re-creation.
type Session = inference.Session

func readMultilineInput(scanner *bufio.Scanner) (string, bool) {
	var lines []string
	for {
		if !scanner.Scan() {
			if len(lines) > 0 {
				return strings.Join(lines, "\n"), true
			}
			return "", false
		}
		line := scanner.Text()
		if strings.HasSuffix(line, "\\") {
			lines = append(lines, strings.TrimSuffix(line, "\\"))
			fmt.Print("... ")
			continue
		}
		lines = append(lines, line)
		return strings.Join(lines, "\n"), true
	}
}

func printHelp() {
	fmt.Println(`Commands:
  /bye, /exit, /quit  Exit the chat
  /clear              Clear conversation context
  /help               Show this help

Tips:
  - End a line with '\' for multiline input
  - Press Ctrl+D to exit`)
}
