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
// Each architecture has a dedicated Go converter. For architectures without
// a Go converter, it falls back to the official llama.cpp convert_hf_to_gguf.py.
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
	ggufArch, ok := detectGGUFArch(hfArch)
	if !ok {
		return ConvertPython(modelDir, progress)
	}

	converter := getConverter(ggufArch, cfg)
	if converter == nil {
		return ConvertPython(modelDir, progress)
	}

	progress("Scanning SafeTensors files", 0, 0)

	stFiles, err := scanSafeTensorsFiles(modelDir)
	if err != nil {
		return "", fmt.Errorf("scanning SafeTensors: %w", err)
	}

	sources := collectTensors(stFiles)

	progress("Parsing tokenizer", 0, 0)

	tok, err := parseTokenizer(modelDir, hfArch)
	if err != nil {
		return "", fmt.Errorf("parsing tokenizer: %w", err)
	}

	if cfg.VocabSize > len(tok.Tokens) {
		for i := len(tok.Tokens); i < cfg.VocabSize; i++ {
			tok.Tokens = append(tok.Tokens, fmt.Sprintf("[PAD%d]", i))
			tok.Scores = append(tok.Scores, -1)
			tok.Types = append(tok.Types, tokenTypeUserDefined)
		}
	}

	progress("Building GGUF", 0, 0)

	writer := newGGUFWriter()
	converter.WriteKV(writer, cfg)
	writeTokenizerKV(writer, tok, cfg)

	if err := converter.ConvertTensors(writer, sources, cfg, progress); err != nil {
		return "", fmt.Errorf("converting tensors: %w", err)
	}

	idx := findKVIndex(writer.kvs, "general.file_type")
	if idx >= 0 {
		writer.kvs[idx].value = uint32(1) // GGML_FTYPE_MOSTLY_F16
	}

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
