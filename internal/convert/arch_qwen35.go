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

		// Conv1d weights: squeeze middle dim=1. PyTorch [out, 1, k] → GGML [k, 1, out] → [k, out].
		if strings.Contains(hfName, "conv1d") && len(dims) == 3 && dims[1] == 1 {
			dims = []uint64{dims[0], dims[2]}
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
			data = qwen35ReorderLinearAttention(data, srcCopy.shape, capturedHFName, capturedGGMLType, cfg)
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

func qwen35ReorderLinearAttention(data []byte, shape []int64, hfName string, dtype GGMLType, cfg *modelConfig) []byte {
	if cfg == nil || !strings.Contains(hfName, "linear_attn.") {
		return data
	}
	numKHeads := cfg.LinearNumKeyHeads
	numVHeads := cfg.LinearNumValueHeads
	headKDim := cfg.LinearKeyHeadDim
	headVDim := cfg.LinearValueHeadDim
	if numKHeads <= 0 || numVHeads <= 0 || headKDim <= 0 || headVDim <= 0 || numVHeads == numKHeads {
		return data
	}
	if numVHeads%numKHeads != 0 {
		return data
	}
	numVPerK := numVHeads / numKHeads

	switch {
	case strings.Contains(hfName, ".in_proj_qkv."):
		qDim := headKDim * numKHeads
		kDim := headKDim * numKHeads
		vDim := headVDim * numVHeads
		return reorderRowsSegment(data, shape, dtype, qDim+kDim, vDim, numKHeads, numVPerK, headVDim)
	case strings.Contains(hfName, ".in_proj_z."):
		return reorderAxisBytes(data, shape, 0, numKHeads, numVPerK, headVDim, dtype)
	case strings.Contains(hfName, ".in_proj_b.") || strings.Contains(hfName, ".in_proj_a."):
		return reorderAxisBytes(data, shape, 0, numKHeads, numVPerK, 1, dtype)
	case strings.Contains(hfName, ".A_log") || strings.Contains(hfName, ".dt_bias") || strings.Contains(hfName, ".dt_proj"):
		axis := len(shape) - 1
		if len(shape) == 1 {
			axis = 0
		}
		return reorderAxisBytes(data, shape, axis, numKHeads, numVPerK, 1, dtype)
	case strings.Contains(hfName, ".conv1d"):
		qkChannels := headKDim * numKHeads * 2
		vChannels := headVDim * numVHeads
		return reorderRowsSegment(data, shape, dtype, qkChannels, vChannels, numKHeads, numVPerK, headVDim)
	case strings.Contains(hfName, ".out_proj."):
		if len(shape) < 2 {
			return data
		}
		return reorderAxisBytes(data, shape, 1, numKHeads, numVPerK, headVDim, dtype)
	default:
		return data
	}
}

func reorderRowsSegment(data []byte, shape []int64, dtype GGMLType, startRows, rowCount, numKHeads, numVPerK, headDim int) []byte {
	if len(shape) == 0 {
		return data
	}
	elemSize := int(dtype.ElementSize())
	if elemSize == 0 {
		return data
	}
	rowWidthElems := int(productInts(shape[1:]))
	if rowWidthElems <= 0 {
		rowWidthElems = 1
	}
	rowWidthBytes := rowWidthElems * elemSize
	totalRows := int(shape[0])
	if startRows < 0 || rowCount <= 0 || startRows+rowCount > totalRows {
		return data
	}
	startByte := startRows * rowWidthBytes
	endByte := (startRows + rowCount) * rowWidthBytes
	if startByte < 0 || endByte > len(data) {
		return data
	}
	out := make([]byte, len(data))
	copy(out, data)
	reordered := reorderAxisBytes(data[startByte:endByte], []int64{int64(rowCount), int64(rowWidthElems)}, 0, numKHeads, numVPerK, headDim, dtype)
	copy(out[startByte:endByte], reordered)
	return out
}

func reorderAxisBytes(data []byte, shape []int64, axis, numKHeads, numVPerK, headDim int, dtype GGMLType) []byte {
	elemSize := int(dtype.ElementSize())
	if elemSize == 0 || len(shape) == 0 {
		return data
	}
	if axis < 0 {
		axis += len(shape)
	}
	if axis < 0 || axis >= len(shape) {
		return data
	}
	expected := numKHeads * numVPerK * headDim
	if int(shape[axis]) != expected {
		return data
	}
	totalElems := int(productInts(shape))
	if totalElems <= 0 || totalElems*elemSize != len(data) {
		return data
	}
	prefix := int(productInts(shape[:axis]))
	if prefix <= 0 {
		prefix = 1
	}
	suffix := int(productInts(shape[axis+1:]))
	if suffix <= 0 {
		suffix = 1
	}
	out := make([]byte, len(data))
	for p := 0; p < prefix; p++ {
		for oldAxis := 0; oldAxis < expected; oldAxis++ {
			kh := oldAxis / (numVPerK * headDim)
			rem := oldAxis % (numVPerK * headDim)
			vpk := rem / headDim
			hd := rem % headDim
			newAxis := (vpk*numKHeads+kh)*headDim + hd
			for s := 0; s < suffix; s++ {
				oldElem := ((p*expected + oldAxis) * suffix) + s
				newElem := ((p*expected + newAxis) * suffix) + s
				copy(out[newElem*elemSize:(newElem+1)*elemSize], data[oldElem*elemSize:(oldElem+1)*elemSize])
			}
		}
	}
	return out
}

func productInts(vals []int64) int64 {
	if len(vals) == 0 {
		return 1
	}
	p := int64(1)
	for _, v := range vals {
		if v <= 0 {
			return 0
		}
		p *= v
	}
	return p
}
