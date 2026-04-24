package inference

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

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
// By default, llama-server output is not mirrored to stderr, but it is still
// captured for diagnostics and appended to the llama-server log file.
func LoadEngine(modelDir string, lm *model.LocalModel) (Engine, error) {
	return LoadEngineWithProgress(modelDir, lm, nil, false, 0, 0, "", "", "")
}

// LoadEngineWithProgress is like LoadEngine but accepts a progress callback
// for SafeTensors → GGUF conversion. When verbose is true, llama-server
// output is printed to stderr.
func LoadEngineWithProgress(modelDir string, lm *model.LocalModel, progress ConvertProgressFunc, verbose bool, numCtx, numParallel int, cacheTypeK, cacheTypeV, dtype string) (Engine, error) {
	normalizedDType, err := convert.NormalizeDType(dtype)
	if err != nil {
		return nil, err
	}

	resolveMMProj := func() (string, error) {
		if path, ok, err := convert.FindMMProjForDType(modelDir, normalizedDType); err != nil {
			return "", err
		} else if ok {
			return path, nil
		}
		if path, ok, err := convert.FindMMProjForDType(modelDir, ""); err != nil {
			return "", err
		} else if ok {
			return path, nil
		}
		return "", nil
	}

	if normalizedDType != "" {
		if ggufPath, ok, err := convert.FindGGUFForDType(modelDir, normalizedDType); err != nil {
			return nil, err
		} else if ok {
			mmproj, err := resolveMMProj()
			if err != nil {
				return nil, err
			}
			return newLlamaEngine(ggufPath, lm.FullName(), verbose, progress, numCtx, numParallel, cacheTypeK, cacheTypeV, mmproj)
		}
		if convert.HasSafeTensors(modelDir) {
			ggufPath, err := convertSafeTensors(modelDir, progress, normalizedDType)
			if err != nil {
				return nil, fmt.Errorf("auto-converting SafeTensors to GGUF: %w", err)
			}
			mmproj, err := resolveMMProj()
			if err != nil {
				return nil, err
			}
			eng, err := newLlamaEngine(ggufPath, lm.FullName(), verbose, progress, numCtx, numParallel, cacheTypeK, cacheTypeV, mmproj)
			if err != nil {
				removeConvertedGGUFIfInvalid(ggufPath, err)
				return nil, err
			}
			return eng, nil
		}
	}

	modelFile, format, err := model.FindModelFile(modelDir)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("finding model file: %w", err)
		}
		if !convert.NeedsConversion(modelDir) {
			return nil, fmt.Errorf("%w: %s", ErrUnsupportedFormat, format)
		}
		format = model.FormatSafeTensors
	}

	switch format {
	case model.FormatGGUF:
		mmproj, err := resolveMMProj()
		if err != nil {
			return nil, err
		}
		return newLlamaEngine(modelFile, lm.FullName(), verbose, progress, numCtx, numParallel, cacheTypeK, cacheTypeV, mmproj)

	case model.FormatSafeTensors:
		ggufPath, err := convertSafeTensors(modelDir, progress, normalizedDType)
		if err != nil {
			return nil, fmt.Errorf("auto-converting SafeTensors to GGUF: %w", err)
		}
		mmproj, err := resolveMMProj()
		if err != nil {
			return nil, err
		}
		eng, err := newLlamaEngine(ggufPath, lm.FullName(), verbose, progress, numCtx, numParallel, cacheTypeK, cacheTypeV, mmproj)
		if err != nil {
			removeConvertedGGUFIfInvalid(ggufPath, err)
			return nil, err
		}
		return eng, nil

	default:
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedFormat, format)
	}
}

func convertSafeTensors(modelDir string, progress ConvertProgressFunc, dtype string) (string, error) {
	if ggufPath, ok, err := convert.FindGGUFForDType(modelDir, dtype); err != nil {
		return "", err
	} else if ok {
		return ggufPath, nil
	}

	var progressFn convert.ProgressFunc
	if progress != nil {
		progressFn = convert.ProgressFunc(progress)
	}

	return convert.Convert(modelDir, progressFn, dtype)
}

func removeConvertedGGUFIfInvalid(ggufPath string, err error) {
	if !shouldRemoveConvertedGGUF(err) {
		log.Printf("keeping converted GGUF after llama-server load failure: %s", ggufPath)
		return
	}
	log.Printf("removing invalid converted GGUF: %s", ggufPath)
	if removeErr := os.Remove(ggufPath); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
		log.Printf("warning: could not remove invalid converted GGUF %s: %v", ggufPath, removeErr)
	}
}

func shouldRemoveConvertedGGUF(err error) bool {
	if err == nil {
		return false
	}

	lower := strings.ToLower(err.Error())

	// Runtime/resource failures should keep the converted file so retries do not
	// pay the conversion cost again.
	keepMarkers := []string{
		"out of memory",
		"cudaMalloc failed",
		"hipmalloc failed",
		"failed to fit params to free device memory",
		"unable to allocate",
		"no such device",
		"device busy",
		"insufficient memory",
		"timeout waiting for llama-server",
		"address already in use",
	}
	for _, marker := range keepMarkers {
		if strings.Contains(lower, strings.ToLower(marker)) {
			return false
		}
	}

	// Only clean up when the failure looks like the GGUF itself is invalid or
	// incomplete, so the next attempt can reconvert a fresh copy.
	removeMarkers := []string{
		"invalid magic characters",
		"invalid gguf",
		"failed to read magic",
		"failed to load model",
		"unknown model architecture",
		"unknown model arch",
		"unknown tensor type",
		"tensor data is not within file bounds",
		"failed to open gguf",
		"gguf file is",
		"not a gguf file",
		"corrupt",
		"truncated",
	}
	for _, marker := range removeMarkers {
		if strings.Contains(lower, strings.ToLower(marker)) {
			return true
		}
	}

	return false
}
