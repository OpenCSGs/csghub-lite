package convert

import (
	"fmt"
	"strings"
)

// ArchConverter handles architecture-specific GGUF conversion logic.
type ArchConverter interface {
	Arch() string
	WriteKV(w *ggufWriter, cfg *modelConfig)
	ConvertTensors(w *ggufWriter, sources []tensorSource, cfg *modelConfig, progress ProgressFunc) error
}

// getConverter returns the appropriate ArchConverter for a GGUF architecture.
// Returns nil only for architectures that have no Go converter and should
// fall back to the Python converter.
func getConverter(ggufArch string, cfg *modelConfig) ArchConverter {
	switch ggufArch {
	// Qwen3.5 hybrid (linear attention + standard attention)
	case "qwen35", "qwen35moe", "qwen3next":
		return &qwen35Converter{arch: ggufArch}

	// Pure SSM
	case "mamba":
		return &mambaConverter{cfg: cfg}
	case "mamba2":
		return &mamba2Converter{cfg: cfg}

	// Hybrid SSM: Mamba + attention
	case "jamba":
		return &jambaConverter{cfg: cfg}
	case "falcon-h1":
		return &hybridConverter{arch: ggufArch, cfg: cfg}
	case "granitehybrid":
		return &hybridConverter{arch: ggufArch, cfg: cfg}
	case "nemotron_h":
		return &hybridConverter{arch: ggufArch, cfg: cfg}
	case "plamo2":
		return &hybridConverter{arch: ggufArch, cfg: cfg}
	case "lfm2", "lfm2moe":
		return &hybridConverter{arch: ggufArch, cfg: cfg}

	// Hybrid linear attention (Kimi-Linear, similar to Qwen3.5)
	case "kimi-linear":
		return &hybridConverter{arch: ggufArch, cfg: cfg, normShift: true}

	// RWKV
	case "rwkv6", "rwkv6qwen2":
		return &rwkv6Converter{arch: ggufArch, cfg: cfg}
	case "rwkv7", "arwkv7":
		return &rwkv7Converter{arch: ggufArch, cfg: cfg}

	// Standard transformer (all other architectures)
	default:
		return &standardConverter{arch: ggufArch}
	}
}

// standardConverter handles transformer architectures with the standard tensor layout.
type standardConverter struct {
	arch string
}

func (c *standardConverter) Arch() string { return c.arch }

func (c *standardConverter) WriteKV(w *ggufWriter, cfg *modelConfig) {
	writeModelKV(w, cfg, c.arch)
}

func (c *standardConverter) ConvertTensors(w *ggufWriter, sources []tensorSource, cfg *modelConfig, progress ProgressFunc) error {
	sources = filterAndTieEmbeddings(sources, cfg)
	nameMapper := tensorNameMapper(c.arch)
	total := len(sources)

	for i, src := range sources {
		ggufName := nameMapper(src.name)
		ggmlType, err := stDTypeToGGML(src.dtype)
		if err != nil {
			return fmt.Errorf("tensor %q: %w", src.name, err)
		}

		dims := reverseShape(src.shape)
		outputType := chooseOutputType(ggufName, ggmlType, len(src.shape))

		srcCopy := src
		capturedGGMLType := ggmlType
		capturedOutputType := outputType
		idx := i

		getData := func() ([]byte, error) {
			progress("Converting tensor", idx+1, total)
			data, err := srcCopy.readData()
			if err != nil {
				return nil, err
			}
			return convertDtype(data, capturedGGMLType, capturedOutputType), nil
		}

		w.addTensor(ggufName, dims, outputType, getData)
	}
	return nil
}

// --- shared helpers ---

func filterAndTieEmbeddings(sources []tensorSource, cfg *modelConfig) []tensorSource {
	var filtered []tensorSource
	for _, t := range sources {
		if shouldIncludeTensor(t.name) {
			filtered = append(filtered, t)
		}
	}
	return handleTiedEmbeddings(filtered, cfg)
}

func chooseOutputType(ggufName string, ggmlType GGMLType, ndims int) GGMLType {
	// 1D tensors (biases, norms, scales) must be F32 for Metal GPU compatibility.
	if ndims <= 1 || isNormTensor(ggufName) {
		return GGMLTypeF32
	}
	if ggmlType == GGMLTypeBF16 {
		return GGMLTypeF16
	}
	return ggmlType
}

func convertDtype(data []byte, from, to GGMLType) []byte {
	if from == GGMLTypeBF16 && to == GGMLTypeF32 {
		return bf16ToF32(data)
	}
	if from == GGMLTypeBF16 && to == GGMLTypeF16 {
		return bf16ToF16(data)
	}
	return data
}

func isNormTensor(name string) bool {
	return strings.HasSuffix(name, "_norm.weight") || strings.HasSuffix(name, ".norm.weight")
}

func chooseOutputTypeForSSM(ggufName string, ggmlType GGMLType, ndims int) GGMLType {
	if ndims <= 1 || strings.HasSuffix(ggufName, "_norm.weight") || strings.HasSuffix(ggufName, ".norm.weight") ||
		strings.HasSuffix(ggufName, ".ssm_norm.weight") {
		return GGMLTypeF32
	}
	if ggmlType == GGMLTypeBF16 {
		return GGMLTypeF16
	}
	return ggmlType
}
