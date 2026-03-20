package convert

import (
	"fmt"
	"math"
	"strings"
)

// qwen35Converter handles Qwen3.5, Qwen3.5MoE, and Qwen3Next architectures.
// These use hybrid attention (standard transformer + linear/SSM layers).
type qwen35Converter struct {
	arch string
}

func (c *qwen35Converter) Arch() string { return c.arch }

func (c *qwen35Converter) WriteKV(w *ggufWriter, cfg *modelConfig) {
	writeModelKV(w, cfg, c.arch)
}

func (c *qwen35Converter) ConvertTensors(w *ggufWriter, sources []tensorSource, cfg *modelConfig, progress ProgressFunc) error {
	sources = filterAndTieEmbeddings(sources, cfg)
	nameMapper := tensorNameMapper(c.arch)
	total := len(sources)

	for i, src := range sources {
		hfName := src.name
		ggmlType, err := stDTypeToGGML(src.dtype)
		if err != nil {
			return fmt.Errorf("tensor %q: %w", hfName, err)
		}

		dims := reverseShape(src.shape)
		srcCopy := src
		idx := i

		// Conv1d weights: squeeze trailing dim=1.
		if strings.Contains(hfName, "conv1d") && len(dims) == 3 && dims[2] == 1 {
			dims = dims[:2]
		}

		ggufName := nameMapper(hfName)
		outputType := chooseOutputTypeForSSM(ggufName, ggmlType, len(src.shape))

		capturedGGMLType := ggmlType
		capturedOutputType := outputType
		capturedHFName := hfName

		getData := func() ([]byte, error) {
			progress("Converting tensor", idx+1, total)
			data, err := srcCopy.readData()
			if err != nil {
				return nil, err
			}
			data = convertDtype(data, capturedGGMLType, capturedOutputType)
			data = qwen35SSMTransform(data, capturedHFName, capturedOutputType)
			return data, nil
		}

		w.addTensor(ggufName, dims, outputType, getData)
	}
	return nil
}

// qwen35SSMTransform applies Qwen3.5-specific tensor transformations:
//   - A_log: negate-exponentiate each value → -exp(x)
//   - norm.weight (non-SSM norm): add 1.0 bias shift
func qwen35SSMTransform(data []byte, hfName string, dtype GGMLType) []byte {
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
