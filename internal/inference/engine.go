package inference

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/opencsgs/csghub-lite/internal/convert"
	"github.com/opencsgs/csghub-lite/internal/model"
)

var ErrUnsupportedFormat = errors.New("unsupported model format for inference")

// Engine is the interface for model inference backends.
type Engine interface {
	// Generate produces text from a prompt, calling onToken for each generated token.
	Generate(ctx context.Context, prompt string, opts Options, onToken TokenCallback) (string, error)

	// Chat produces a response from a conversation history.
	Chat(ctx context.Context, messages []Message, opts Options, onToken TokenCallback) (string, error)

	// Close releases the model resources.
	Close() error

	// ModelName returns the loaded model identifier.
	ModelName() string
}

// ChatCompletionProxier exposes direct access to the underlying
// OpenAI-compatible /v1/chat/completions API for advanced use cases
// such as native Ollama tool-calling compatibility.
type ChatCompletionProxier interface {
	ChatCompletion(ctx context.Context, reqBody map[string]interface{}) (*http.Response, error)
}

// ConvertProgressFunc receives conversion progress updates.
// If nil, conversion progress is not reported.
type ConvertProgressFunc func(step string, current, total int)

// LoadEngine loads a model and returns an Engine for inference.
// If the model is SafeTensors, it auto-converts to GGUF first.
// The llama-server output is suppressed by default.
func LoadEngine(modelDir string, lm *model.LocalModel) (Engine, error) {
	return LoadEngineWithProgress(modelDir, lm, nil, false, 0)
}

// LoadEngineWithProgress is like LoadEngine but accepts a progress callback
// for SafeTensors → GGUF conversion. When verbose is true, llama-server
// output is printed to stderr.
func LoadEngineWithProgress(modelDir string, lm *model.LocalModel, progress ConvertProgressFunc, verbose bool, numCtx int) (Engine, error) {
	modelFile, format, err := model.FindModelFile(modelDir)
	if err != nil {
		return nil, fmt.Errorf("finding model file: %w", err)
	}

	mmproj := model.FindMMProj(modelDir)

	switch format {
	case model.FormatGGUF:
		return newLlamaEngine(modelFile, lm.FullName(), verbose, progress, numCtx, mmproj)

	case model.FormatSafeTensors:
		ggufPath, err := convertSafeTensors(modelDir, progress)
		if err != nil {
			return nil, fmt.Errorf("auto-converting SafeTensors to GGUF: %w", err)
		}
		eng, err := newLlamaEngine(ggufPath, lm.FullName(), verbose, progress, numCtx, mmproj)
		if err != nil {
			log.Printf("removing invalid converted GGUF: %s", ggufPath)
			os.Remove(ggufPath)
			return nil, err
		}
		return eng, nil

	default:
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedFormat, format)
	}
}

func convertSafeTensors(modelDir string, progress ConvertProgressFunc) (string, error) {
	if ggufPath, ok := convert.HasGGUF(modelDir); ok {
		return ggufPath, nil
	}

	var progressFn convert.ProgressFunc
	if progress != nil {
		progressFn = convert.ProgressFunc(progress)
	}

	return convert.Convert(modelDir, progressFn)
}
