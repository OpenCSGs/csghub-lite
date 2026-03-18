package convert

import (
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
)

// ProgressFunc reports conversion progress.
type ProgressFunc func(step string, current, total int)

// Convert converts SafeTensors model files in modelDir to a GGUF file.
// For complex architectures (SSM, Mamba, etc.), it delegates to the official
// llama.cpp convert_hf_to_gguf.py if Python is available. For simple
// architectures, it uses the built-in Go converter.
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
	ggufArch, needsPython := detectGGUFArch(hfArch)
	if ggufArch == "" {
		return "", fmt.Errorf("unsupported architecture: %s", hfArch)
	}

	if needsPython {
		return ConvertPython(modelDir, progress)
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

	needsSSMTransform := isSSMArch(ggufArch)

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
		hfName := src.name

		// SSM architectures: conv1d weights need dimension squeeze.
		if needsSSMTransform && strings.Contains(hfName, "conv1d") && len(dims) == 3 && dims[2] == 1 {
			dims = dims[:2]
		}

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

		// Capture loop state for the closure.
		capturedGGMLType := ggmlType
		capturedOutputType := outputType
		capturedSSM := needsSSMTransform
		capturedHFName := hfName

		getData := func() ([]byte, error) {
			progress("Converting tensor", i+1, totalTensors)
			data, err := srcCopy.readData()
			if err != nil {
				return nil, err
			}
			if capturedGGMLType == GGMLTypeBF16 && capturedOutputType == GGMLTypeF32 {
				data = bf16ToF32(data)
			} else if capturedGGMLType == GGMLTypeBF16 && capturedOutputType == GGMLTypeF16 {
				data = bf16ToF16(data)
			}
			if capturedSSM {
				data = applySSMTransform(data, capturedHFName, capturedOutputType)
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
		lower := strings.ToLower(e.Name())
		if !e.IsDir() && strings.HasSuffix(lower, ".gguf") && !strings.Contains(lower, "mmproj") {
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

func isSSMArch(arch string) bool {
	switch arch {
	case "qwen35", "qwen35moe", "qwen3next":
		return true
	}
	return false
}

// applySSMTransform applies architecture-specific data transformations for SSM models.
// - A_log tensors: negate-exponentiate each value (-exp(x))
// - Non-SSM norm weights: add 1.0 to each value (Qwen3Next convention)
func applySSMTransform(data []byte, hfName string, dtype GGMLType) []byte {
	if strings.HasSuffix(hfName, ".A_log") {
		return transformFloats(data, dtype, func(v float32) float32 {
			return float32(-math.Exp(float64(v)))
		})
	}
	if strings.HasSuffix(hfName, "norm.weight") && !strings.Contains(hfName, "linear_attn.norm") {
		return transformFloats(data, dtype, func(v float32) float32 {
			return v + 1.0
		})
	}
	return data
}

// transformFloats applies fn to each float value in data (F16 or F32 encoded).
func transformFloats(data []byte, dtype GGMLType, fn func(float32) float32) []byte {
	switch dtype {
	case GGMLTypeF32:
		out := make([]byte, len(data))
		for i := 0; i+3 < len(data); i += 4 {
			v := math.Float32frombits(binary.LittleEndian.Uint32(data[i:]))
			binary.LittleEndian.PutUint32(out[i:], math.Float32bits(fn(v)))
		}
		return out
	case GGMLTypeF16:
		out := make([]byte, len(data))
		for i := 0; i+1 < len(data); i += 2 {
			f16 := binary.LittleEndian.Uint16(data[i:])
			v := float16ToFloat32(f16)
			binary.LittleEndian.PutUint16(out[i:], float32ToFloat16(fn(v)))
		}
		return out
	}
	return data
}

func float16ToFloat32(h uint16) float32 {
	sign := uint32(h>>15) & 1
	exp := uint32(h>>10) & 0x1f
	mant := uint32(h) & 0x3ff
	if exp == 0 {
		if mant == 0 {
			return math.Float32frombits(sign << 31)
		}
		for mant&0x400 == 0 {
			mant <<= 1
			exp--
		}
		exp++
		mant &= 0x3ff
	} else if exp == 31 {
		return math.Float32frombits(sign<<31 | 0x7f800000 | mant<<13)
	}
	return math.Float32frombits(sign<<31 | (exp+112)<<23 | mant<<13)
}

func float32ToFloat16(f float32) uint16 {
	bits := math.Float32bits(f)
	return float32BitsToFloat16(bits)
}
