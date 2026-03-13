package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/opencsgs/csghub-lite/internal/config"
	"github.com/opencsgs/csghub-lite/internal/csghub"
	"github.com/opencsgs/csghub-lite/internal/inference"
	"github.com/opencsgs/csghub-lite/internal/model"
	"github.com/spf13/cobra"
)

func newRunCmd() *cobra.Command {
	var verbose bool

	cmd := &cobra.Command{
		Use:   "run MODEL",
		Short: "Download (if needed) and chat with a model",
		Long: `Download a model from CSGHub if not already present, then start an interactive
chat session. Type your message and press Enter to send. Use '/bye' to exit.

Multiline input: end a line with '\' to continue on the next line.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRun(cmd, args, verbose)
		},
	}

	cmd.Flags().BoolVar(&verbose, "verbose", false, "show detailed llama-server output")
	return cmd
}

func runRun(cmd *cobra.Command, args []string, verbose bool) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	mgr := model.NewManager(cfg)
	modelID := args[0]

	// Pull model if not present
	if !mgr.Exists(modelID) {
		fmt.Printf("Model %s not found locally. Pulling from %s...\n", modelID, cfg.ServerURL)
		var lastFile string
		progress := func(p csghub.SnapshotProgress) {
			if p.FileName != lastFile {
				if lastFile != "" {
					fmt.Println()
				}
				lastFile = p.FileName
			}
			if p.BytesTotal > 0 {
				pct := float64(p.BytesCompleted) / float64(p.BytesTotal) * 100
				fmt.Printf("\r  [%d/%d] %s  %.1f%%",
					p.FileIndex+1, p.TotalFiles, p.FileName, pct)
			}
		}

		if _, err := mgr.Pull(cmd.Context(), modelID, progress); err != nil {
			return fmt.Errorf("pull failed: %w", err)
		}
		fmt.Println("\nPull complete.")
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

func convertProgress(step string, current, total int) {
	if total > 0 {
		fmt.Printf("\r  Converting SafeTensors → GGUF: %s [%d/%d]", step, current, total)
		if current == total {
			fmt.Println()
		}
	} else {
		fmt.Printf("  %s...\n", step)
	}
}
