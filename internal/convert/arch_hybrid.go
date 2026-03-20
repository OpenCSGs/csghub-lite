package convert

import (
	"fmt"
	"math"
	"strings"
)

// hybridConverter handles hybrid architectures that combine standard
// transformer attention layers with Mamba/SSM recurrent layers.
// Covers: falcon-h1, granitehybrid (Bamba), plamo2, lfm2/lfm2moe,
// nemotron_h, kimi-linear.
type hybridConverter struct {
	arch       string
	cfg        *modelConfig
	normShift  bool // apply +1.0 to non-SSM norm weights (Qwen3Next/kimi-linear style)
}

func (c *hybridConverter) Arch() string { return c.arch }

func (c *hybridConverter) WriteKV(w *ggufWriter, cfg *modelConfig) {
	writeModelKV(w, cfg, c.arch)
	c.writeSSMKV(w, cfg)
}

func (c *hybridConverter) writeSSMKV(w *ggufWriter, cfg *modelConfig) {
	prefix := c.arch

	if cfg.ConvKernel > 0 {
		w.addKV(prefix+".ssm.conv_kernel", uint32(cfg.ConvKernel))
	}
	if cfg.StateSize > 0 {
		w.addKV(prefix+".ssm.state_size", uint32(cfg.StateSize))
	}

	innerSize := cfg.MambaDSSM
	if innerSize == 0 {
		innerSize = cfg.IntermediateSize
	}
	if innerSize > 0 {
		w.addKV(prefix+".ssm.inner_size", uint32(innerSize))
	}

	if cfg.TimeStepRank > 0 {
		w.addKV(prefix+".ssm.time_step_rank", uint32(cfg.TimeStepRank))
	}
	if cfg.NGroups > 0 {
		w.addKV(prefix+".ssm.group_count", uint32(cfg.NGroups))
	}
	if cfg.NumLocalExperts > 0 {
		w.addKV(prefix+".expert_count", uint32(cfg.NumLocalExperts))
	}
	if cfg.NumExpertsPerTok > 0 {
		w.addKV(prefix+".expert_used_count", uint32(cfg.NumExpertsPerTok))
	}
}

func (c *hybridConverter) ConvertTensors(w *ggufWriter, sources []tensorSource, cfg *modelConfig, progress ProgressFunc) error {
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

		// Conv1d squeeze for SSM layers
		if strings.Contains(hfName, "conv1d") && len(dims) == 3 && dims[2] == 1 {
			dims = dims[:2]
		}

		ggufName := nameMapper(hfName)
		outputType := chooseOutputTypeForSSM(ggufName, ggmlType, len(src.shape))

		capturedGGMLType := ggmlType
		capturedOutputType := outputType
		capturedHFName := hfName
		normShift := c.normShift

		getData := func() ([]byte, error) {
			progress("Converting tensor", idx+1, total)
			data, err := srcCopy.readData()
			if err != nil {
				return nil, err
			}
			data = convertDtype(data, capturedGGMLType, capturedOutputType)
			data = hybridSSMTransform(data, capturedHFName, capturedOutputType, normShift)
			return data, nil
		}

		w.addTensor(ggufName, dims, outputType, getData)
	}
	return nil
}

// hybridSSMTransform applies common SSM tensor transformations:
//   - A_log: negate-exponentiate → -exp(x)
//   - norm.weight: optionally add 1.0 bias shift (when normShift is true)
func hybridSSMTransform(data []byte, hfName string, dtype GGMLType, normShift bool) []byte {
	if strings.HasSuffix(hfName, ".A_log") {
		return transformFloats(data, dtype, func(v float32) float32 {
			return float32(-math.Exp(float64(v)))
		})
	}
	if normShift && strings.HasSuffix(hfName, "norm.weight") {
		// Skip SSM internal norm (mamba.norm, linear_attn.norm)
		if !strings.Contains(hfName, ".mamba.norm") &&
			!strings.Contains(hfName, ".mamba2.norm") &&
			!strings.Contains(hfName, "linear_attn.norm") {
			return transformFloats(data, dtype, func(v float32) float32 {
				return v + 1.0
			})
		}
	}
	return data
}
