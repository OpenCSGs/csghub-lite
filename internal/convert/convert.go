package convert

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ProgressFunc reports conversion progress.
type ProgressFunc func(step string, current, total int)

// Convert converts SafeTensors model files in modelDir to a GGUF file.
// Returns the path to the generated GGUF file.
func Convert(modelDir string, progress ProgressFunc) (string, error) {
	if progress == nil {
		progress = func(string, int, int) {}
	}

	progress("Reading model configuration", 0, 0)

	cfg, err := loadModelConfig(modelDir)
	if err != nil {
		return "", fmt.Errorf("loading config: %w", err)
	}

	hfArch := cfg.Architectures[0]
	ggufArch, err := detectGGUFArch(hfArch)
	if err != nil {
		return "", err
	}

	progress("Scanning SafeTensors files", 0, 0)

	stFiles, err := scanSafeTensorsFiles(modelDir)
	if err != nil {
		return "", fmt.Errorf("scanning SafeTensors: %w", err)
	}

	sources := collectTensors(stFiles)

	// Filter out tensors that llama.cpp regenerates.
	var filtered []tensorSource
	for _, t := range sources {
		if shouldIncludeTensor(t.name) {
			filtered = append(filtered, t)
		}
	}
	sources = filtered

	// Handle tied embeddings (lm_head shares weights with embed_tokens).
	sources = handleTiedEmbeddings(sources, cfg)

	progress("Parsing tokenizer", 0, 0)

	tok, err := parseTokenizer(modelDir, hfArch)
	if err != nil {
		return "", fmt.Errorf("parsing tokenizer: %w", err)
	}

	// Pad vocabulary if needed.
	if cfg.VocabSize > len(tok.Tokens) {
		for i := len(tok.Tokens); i < cfg.VocabSize; i++ {
			tok.Tokens = append(tok.Tokens, fmt.Sprintf("[PAD%d]", i))
			tok.Scores = append(tok.Scores, -1)
			tok.Types = append(tok.Types, tokenTypeUserDefined)
		}
	}

	// Build GGUF.
	progress("Building GGUF", 0, 0)

	nameMapper := tensorNameMapper(ggufArch)
	writer := newGGUFWriter()

	// Write model metadata.
	writeModelKV(writer, cfg, ggufArch)

	// Write tokenizer metadata.
	writeTokenizerKV(writer, tok, cfg)

	// Add tensors. Norm/1D weights → F32 for stability; large matrices → F16.
	totalTensors := len(sources)
	for i, src := range sources {
		ggufName := nameMapper(src.name)
		ggmlType, err := stDTypeToGGML(src.dtype)
		if err != nil {
			return "", fmt.Errorf("tensor %q: %w", src.name, err)
		}

		dims := reverseShape(src.shape)
		srcCopy := src

		isNorm := isNormTensor(ggufName)
		var outputType GGMLType
		switch {
		case isNorm:
			outputType = GGMLTypeF32
		case ggmlType == GGMLTypeBF16:
			outputType = GGMLTypeF16
		default:
			outputType = ggmlType
		}

		getData := func() ([]byte, error) {
			progress("Converting tensor", i+1, totalTensors)
			data, err := srcCopy.readData()
			if err != nil {
				return nil, err
			}
			if ggmlType == GGMLTypeBF16 && outputType == GGMLTypeF32 {
				data = bf16ToF32(data)
			} else if ggmlType == GGMLTypeBF16 && outputType == GGMLTypeF16 {
				data = bf16ToF16(data)
			}
			return data, nil
		}

		writer.addTensor(ggufName, dims, outputType, getData)
	}

	idx := findKVIndex(writer.kvs, "general.file_type")
	if idx >= 0 {
		writer.kvs[idx].value = uint32(1) // GGML_FTYPE_MOSTLY_F16
	}

	// Write to temp file, then rename.
	outputName := generateOutputName(modelDir, cfg)
	outputPath := filepath.Join(modelDir, outputName)
	tmpPath := outputPath + ".tmp"

	progress("Writing GGUF file", 0, 0)

	if err := writer.writeTo(tmpPath); err != nil {
		os.Remove(tmpPath)
		return "", fmt.Errorf("writing GGUF: %w", err)
	}

	if err := os.Rename(tmpPath, outputPath); err != nil {
		os.Remove(tmpPath)
		return "", fmt.Errorf("finalizing GGUF: %w", err)
	}

	return outputPath, nil
}

// HasGGUF checks if a GGUF file already exists in the model directory.
func HasGGUF(modelDir string) (string, bool) {
	entries, err := os.ReadDir(modelDir)
	if err != nil {
		return "", false
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(strings.ToLower(e.Name()), ".gguf") {
			return filepath.Join(modelDir, e.Name()), true
		}
	}
	return "", false
}

// NeedsConversion checks if the model directory contains SafeTensors files
// but no GGUF files.
func NeedsConversion(modelDir string) bool {
	_, hasGGUF := HasGGUF(modelDir)
	if hasGGUF {
		return false
	}

	entries, err := os.ReadDir(modelDir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(strings.ToLower(e.Name()), ".safetensors") {
			return true
		}
	}
	return false
}

func reverseShape(shape []int64) []uint64 {
	n := len(shape)
	dims := make([]uint64, n)
	for i, s := range shape {
		dims[n-1-i] = uint64(s)
	}
	return dims
}

func generateOutputName(modelDir string, cfg *modelConfig) string {
	base := filepath.Base(modelDir)
	if base == "." || base == "" {
		base = "model"
	}
	return base + "-f16.gguf"
}

func findKVIndex(kvs []ggufKV, key string) int {
	for i, kv := range kvs {
		if kv.key == key {
			return i
		}
	}
	return -1
}

func isNormTensor(name string) bool {
	return strings.HasSuffix(name, "_norm.weight")
}
