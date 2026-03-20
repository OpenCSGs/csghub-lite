package convert

import (
	"fmt"
	"math"
	"strings"
)

// --- Mamba-1 ---

type mambaConverter struct {
	cfg *modelConfig
}

func (c *mambaConverter) Arch() string { return "mamba" }

func (c *mambaConverter) WriteKV(w *ggufWriter, cfg *modelConfig) {
	prefix := "mamba"
	w.addKV("general.architecture", prefix)
	w.addKV("general.name", cfg.ModelType)
	w.addKV("general.file_type", uint32(1))

	w.addKV(prefix+".block_count", uint32(cfg.NumHiddenLayers))
	w.addKV(prefix+".context_length", uint32(1<<20))
	w.addKV(prefix+".embedding_length", uint32(cfg.HiddenSize))
	w.addKV(prefix+".feed_forward_length", uint32(0))
	w.addKV(prefix+".attention.head_count", uint32(0))
	w.addKV(prefix+".attention.layer_norm_rms_epsilon", float32(cfg.RmsNormEps))

	convKernel := cfg.ConvKernel
	if convKernel == 0 {
		convKernel = 4
	}
	w.addKV(prefix+".ssm.conv_kernel", uint32(convKernel))

	innerSize := cfg.IntermediateSize
	if innerSize == 0 {
		innerSize = cfg.HiddenSize * 2
	}
	w.addKV(prefix+".ssm.inner_size", uint32(innerSize))

	stateSize := cfg.StateSize
	if stateSize == 0 {
		stateSize = 16
	}
	w.addKV(prefix+".ssm.state_size", uint32(stateSize))

	dtRank := cfg.TimeStepRank
	if dtRank == 0 {
		dtRank = (cfg.HiddenSize + 15) / 16
	}
	w.addKV(prefix+".ssm.time_step_rank", uint32(dtRank))
}

func (c *mambaConverter) ConvertTensors(w *ggufWriter, sources []tensorSource, cfg *modelConfig, progress ProgressFunc) error {
	sources = filterAndTieEmbeddings(sources, cfg)
	nameMapper := mambaNameMapper()
	total := len(sources)

	// Detect tied embeddings: check if output.weight data equals token_embd.weight
	var embedSrc *tensorSource
	for i := range sources {
		mapped := nameMapper(sources[i].name)
		if strings.HasPrefix(mapped, "token_embd") {
			embedSrc = &sources[i]
			break
		}
	}

	for i, src := range sources {
		hfName := src.name
		ggmlType, err := stDTypeToGGML(src.dtype)
		if err != nil {
			return fmt.Errorf("tensor %q: %w", hfName, err)
		}

		dims := reverseShape(src.shape)
		ggufName := nameMapper(hfName)

		// Skip duplicate output tensor if tied
		if strings.HasPrefix(ggufName, "output") && cfg.TieWordEmbeddings && embedSrc != nil {
			continue
		}

		// Conv1d squeeze. PyTorch [out, 1, k] → GGML [k, 1, out] → [k, out].
		if strings.Contains(ggufName, "ssm_conv1d") && len(dims) == 3 && dims[1] == 1 {
			dims = []uint64{dims[0], dims[2]}
		}

		outputType := chooseOutputTypeForSSM(ggufName, ggmlType, len(src.shape))
		srcCopy := src
		capturedGGMLType := ggmlType
		capturedOutputType := outputType
		capturedHFName := hfName
		idx := i

		getData := func() ([]byte, error) {
			progress("Converting tensor", idx+1, total)
			data, err := srcCopy.readData()
			if err != nil {
				return nil, err
			}
			data = convertDtype(data, capturedGGMLType, capturedOutputType)
			if strings.HasSuffix(capturedHFName, ".A_log") || strings.HasSuffix(capturedHFName, "A_log") {
				data = transformFloats(data, capturedOutputType, func(v float32) float32 {
					return float32(-math.Exp(float64(v)))
				})
			}
			return data, nil
		}

		w.addTensor(ggufName, dims, outputType, getData)
	}
	return nil
}

func mambaNameMapper() func(string) string {
	r := strings.NewReplacer(
		"backbone.embedding", "token_embd",
		"backbone.layers.", "blk.",
		"backbone.norm_f", "output_norm",
		"model.embed_tokens", "token_embd",
		"model.layers.", "blk.",
		"model.norm", "output_norm",
		".mixer.in_proj", ".ssm_in",
		".mixer.conv1d", ".ssm_conv1d",
		".mixer.x_proj", ".ssm_x",
		".mixer.dt_proj", ".ssm_dt",
		".mixer.A_log", ".ssm_a",
		".mixer.D", ".ssm_d",
		".mixer.out_proj", ".ssm_out",
		".norm", ".attn_norm",
		"backbone.lm_head", "output",
		"lm_head", "output",
	)
	return func(hfName string) string {
		hfName = strings.TrimPrefix(hfName, "model.backbone.")
		return r.Replace(hfName)
	}
}

// --- Mamba-2 ---

type mamba2Converter struct {
	cfg *modelConfig
}

func (c *mamba2Converter) Arch() string { return "mamba2" }

func (c *mamba2Converter) WriteKV(w *ggufWriter, cfg *modelConfig) {
	prefix := "mamba2"
	w.addKV("general.architecture", prefix)
	w.addKV("general.name", cfg.ModelType)
	w.addKV("general.file_type", uint32(1))

	w.addKV(prefix+".block_count", uint32(cfg.NumHiddenLayers))
	w.addKV(prefix+".context_length", uint32(1<<20))
	w.addKV(prefix+".embedding_length", uint32(cfg.HiddenSize))
	w.addKV(prefix+".feed_forward_length", uint32(0))
	w.addKV(prefix+".attention.head_count", uint32(0))
	w.addKV(prefix+".attention.layer_norm_rms_epsilon", float32(cfg.RmsNormEps))

	convKernel := cfg.ConvKernel
	if convKernel == 0 {
		convKernel = 4
	}
	w.addKV(prefix+".ssm.conv_kernel", uint32(convKernel))

	innerSize := cfg.MambaDSSM
	if innerSize == 0 {
		innerSize = cfg.IntermediateSize
	}
	if innerSize == 0 {
		innerSize = cfg.HiddenSize * 2
	}
	w.addKV(prefix+".ssm.inner_size", uint32(innerSize))

	stateSize := cfg.StateSize
	if stateSize == 0 {
		stateSize = 128
	}
	w.addKV(prefix+".ssm.state_size", uint32(stateSize))

	headDim := cfg.HeadDim
	if headDim == 0 {
		headDim = 64
	}
	dtRank := innerSize / headDim
	w.addKV(prefix+".ssm.time_step_rank", uint32(dtRank))

	nGroups := cfg.NGroups
	if nGroups == 0 {
		nGroups = 1
	}
	w.addKV(prefix+".ssm.group_count", uint32(nGroups))
}

func (c *mamba2Converter) ConvertTensors(w *ggufWriter, sources []tensorSource, cfg *modelConfig, progress ProgressFunc) error {
	sources = filterAndTieEmbeddings(sources, cfg)
	nameMapper := mamba2NameMapper()
	total := len(sources)

	nGroups := cfg.NGroups
	if nGroups == 0 {
		nGroups = 1
	}

	for i, src := range sources {
		hfName := src.name
		ggmlType, err := stDTypeToGGML(src.dtype)
		if err != nil {
			return fmt.Errorf("tensor %q: %w", hfName, err)
		}

		dims := reverseShape(src.shape)
		ggufName := nameMapper(hfName)

		// Conv1d squeeze. PyTorch [out, 1, k] → GGML [k, 1, out] → [k, out].
		if strings.Contains(ggufName, "ssm_conv1d") && len(dims) == 3 && dims[1] == 1 {
			dims = []uint64{dims[0], dims[2]}
		}

		// SSM_A and SSM_D: unsqueeze (add trailing dim=1)
		if strings.HasSuffix(ggufName, ".ssm_a") || strings.HasSuffix(ggufName, ".ssm_d") {
			dims = append([]uint64{1}, dims...)
		}

		// SSM_NORM: reshape to (n_groups, d_inner/n_groups)
		if strings.HasSuffix(ggufName, ".ssm_norm.weight") && len(dims) == 1 && nGroups > 1 {
			dInner := dims[0]
			dims = []uint64{dInner / uint64(nGroups), uint64(nGroups)}
		}

		outputType := chooseOutputTypeForSSM(ggufName, ggmlType, len(src.shape))
		srcCopy := src
		capturedGGMLType := ggmlType
		capturedOutputType := outputType
		capturedHFName := hfName
		idx := i

		getData := func() ([]byte, error) {
			progress("Converting tensor", idx+1, total)
			data, err := srcCopy.readData()
			if err != nil {
				return nil, err
			}
			data = convertDtype(data, capturedGGMLType, capturedOutputType)
			if strings.HasSuffix(capturedHFName, ".A_log") || strings.HasSuffix(capturedHFName, "A_log") {
				data = transformFloats(data, capturedOutputType, func(v float32) float32 {
					return float32(-math.Exp(float64(v)))
				})
			}
			return data, nil
		}

		w.addTensor(ggufName, dims, outputType, getData)
	}
	return nil
}

func mamba2NameMapper() func(string) string {
	r := strings.NewReplacer(
		"backbone.embedding", "token_embd",
		"backbone.layers.", "blk.",
		"backbone.norm_f", "output_norm",
		"model.embed_tokens", "token_embd",
		"model.layers.", "blk.",
		"model.norm", "output_norm",
		".mixer.in_proj", ".ssm_in",
		".mixer.conv1d", ".ssm_conv1d",
		".mixer.dt_bias", ".ssm_dt.bias",
		".mixer.A_log", ".ssm_a",
		".mixer.D", ".ssm_d",
		".mixer.norm", ".ssm_norm",
		".mixer.out_proj", ".ssm_out",
		".norm", ".attn_norm",
		"backbone.lm_head", "output",
		"lm_head", "output",
	)
	return func(hfName string) string {
		hfName = strings.TrimPrefix(hfName, "model.backbone.")
		return r.Replace(hfName)
	}
}

// --- Jamba (Mamba + attention hybrid) ---

type jambaConverter struct {
	cfg *modelConfig
}

func (c *jambaConverter) Arch() string { return "jamba" }

func (c *jambaConverter) WriteKV(w *ggufWriter, cfg *modelConfig) {
	prefix := "jamba"
	w.addKV("general.architecture", prefix)
	w.addKV("general.name", cfg.ModelType)
	w.addKV("general.file_type", uint32(1))

	w.addKV(prefix+".block_count", uint32(cfg.NumHiddenLayers))
	w.addKV(prefix+".context_length", uint32(cfg.MaxPositionEmbeddings))
	w.addKV(prefix+".embedding_length", uint32(cfg.HiddenSize))
	w.addKV(prefix+".feed_forward_length", uint32(cfg.IntermediateSize))
	w.addKV(prefix+".attention.head_count", uint32(cfg.NumAttentionHeads))
	w.addKV(prefix+".attention.head_count_kv", uint32(cfg.NumKeyValueHeads))
	w.addKV(prefix+".attention.layer_norm_rms_epsilon", float32(cfg.RmsNormEps))
	w.addKV(prefix+".rope.freq_base", float32(cfg.RopeTheta))

	convKernel := cfg.ConvKernel
	if convKernel == 0 {
		convKernel = 4
	}
	w.addKV(prefix+".ssm.conv_kernel", uint32(convKernel))

	innerSize := cfg.IntermediateSize
	if innerSize == 0 {
		innerSize = cfg.HiddenSize * 2
	}
	w.addKV(prefix+".ssm.inner_size", uint32(innerSize))

	stateSize := cfg.StateSize
	if stateSize == 0 {
		stateSize = 16
	}
	w.addKV(prefix+".ssm.state_size", uint32(stateSize))

	dtRank := cfg.TimeStepRank
	if dtRank == 0 {
		dtRank = (cfg.HiddenSize + 15) / 16
	}
	w.addKV(prefix+".ssm.time_step_rank", uint32(dtRank))

	if cfg.NumLocalExperts > 0 {
		w.addKV(prefix+".expert_count", uint32(cfg.NumLocalExperts))
	}
	if cfg.NumExpertsPerTok > 0 {
		w.addKV(prefix+".expert_used_count", uint32(cfg.NumExpertsPerTok))
	}
}

func (c *jambaConverter) ConvertTensors(w *ggufWriter, sources []tensorSource, cfg *modelConfig, progress ProgressFunc) error {
	sources = filterAndTieEmbeddings(sources, cfg)
	nameMapper := jambaNameMapper()
	total := len(sources)

	for i, src := range sources {
		hfName := src.name
		ggmlType, err := stDTypeToGGML(src.dtype)
		if err != nil {
			return fmt.Errorf("tensor %q: %w", hfName, err)
		}

		dims := reverseShape(src.shape)
		ggufName := nameMapper(hfName)

		if strings.Contains(ggufName, "ssm_conv1d") && len(dims) == 3 && dims[1] == 1 {
			dims = []uint64{dims[0], dims[2]}
		}

		outputType := chooseOutputTypeForSSM(ggufName, ggmlType, len(src.shape))
		srcCopy := src
		capturedGGMLType := ggmlType
		capturedOutputType := outputType
		capturedHFName := hfName
		idx := i

		getData := func() ([]byte, error) {
			progress("Converting tensor", idx+1, total)
			data, err := srcCopy.readData()
			if err != nil {
				return nil, err
			}
			data = convertDtype(data, capturedGGMLType, capturedOutputType)
			if strings.HasSuffix(capturedHFName, ".A_log") {
				data = transformFloats(data, capturedOutputType, func(v float32) float32 {
					return float32(-math.Exp(float64(v)))
				})
			}
			return data, nil
		}

		w.addTensor(ggufName, dims, outputType, getData)
	}
	return nil
}

func jambaNameMapper() func(string) string {
	r := strings.NewReplacer(
		"model.embed_tokens", "token_embd",
		"model.layers.", "blk.",
		"model.norm", "output_norm",
		"model.final_layernorm", "output_norm",
		// Attention tensors (standard transformer layers in Jamba)
		".self_attn.q_proj", ".attn_q",
		".self_attn.k_proj", ".attn_k",
		".self_attn.v_proj", ".attn_v",
		".self_attn.o_proj", ".attn_output",
		// MLP tensors
		".mlp.gate_proj", ".ffn_gate",
		".mlp.up_proj", ".ffn_up",
		".mlp.down_proj", ".ffn_down",
		// Mamba SSM tensors
		".mamba.in_proj", ".ssm_in",
		".mamba.conv1d", ".ssm_conv1d",
		".mamba.x_proj", ".ssm_x",
		".mamba.dt_proj", ".ssm_dt",
		".mamba.A_log", ".ssm_a",
		".mamba.D", ".ssm_d",
		".mamba.out_proj", ".ssm_out",
		// Norm
		".input_layernorm", ".attn_norm",
		".pre_mamba_layernorm", ".attn_norm",
		".post_attention_layernorm", ".ffn_norm",
		"lm_head", "output",
	)
	return func(hfName string) string {
		return r.Replace(hfName)
	}
}
