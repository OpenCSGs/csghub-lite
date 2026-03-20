package convert

import (
	"encoding/binary"
	"fmt"
	"math"
	"sort"
	"strings"
)

// --- RWKV6 ---

type rwkv6Converter struct {
	arch string
	cfg  *modelConfig
}

func (c *rwkv6Converter) Arch() string { return c.arch }

func (c *rwkv6Converter) WriteKV(w *ggufWriter, cfg *modelConfig) {
	prefix := c.arch
	w.addKV("general.architecture", prefix)
	w.addKV("general.name", cfg.ModelType)
	w.addKV("general.file_type", uint32(1))

	w.addKV(prefix+".block_count", uint32(cfg.NumHiddenLayers))
	w.addKV(prefix+".context_length", uint32(1<<20))
	w.addKV(prefix+".embedding_length", uint32(cfg.HiddenSize))
	w.addKV(prefix+".attention.head_count", uint32(0))
	w.addKV(prefix+".attention.layer_norm_epsilon", float32(cfg.RmsNormEps))

	ffn := cfg.IntermediateSize
	if ffn == 0 {
		ffn = int(float64(cfg.HiddenSize)*3.5) / 32 * 32
	}
	w.addKV(prefix+".feed_forward_length", uint32(ffn))

	headSize := cfg.WkvHeadSize
	if headSize == 0 {
		headSize = 64
	}
	w.addKV(prefix+".wkv_head_size", uint32(headSize))

	rescale := cfg.RescaleEvery
	if rescale > 0 {
		w.addKV(prefix+".rescale_every_n_layers", uint32(rescale))
	}

	tmExtraDim := 32
	if cfg.HiddenSize >= 4096 {
		tmExtraDim = 64
	}
	w.addKV(prefix+".time_mix_extra_dim", uint32(tmExtraDim))

	tdExtraDim := 64
	if cfg.HiddenSize >= 4096 {
		tdExtraDim = 128
	}
	w.addKV(prefix+".time_decay_extra_dim", uint32(tdExtraDim))
}

func (c *rwkv6Converter) ConvertTensors(w *ggufWriter, sources []tensorSource, cfg *modelConfig, progress ProgressFunc) error {
	sources = filterAndTieEmbeddings(sources, cfg)
	nameMapper := rwkv6NameMapper()
	total := len(sources)

	rescale := cfg.RescaleEvery

	// Collect lerp tensors for fusion: block_id → {w,k,v,r,g} → tensorSource
	type lerpKey struct {
		block int
		kind  string
	}
	lerpMap := make(map[lerpKey]tensorSource)
	lerpBlocks := make(map[int]int) // block_id → count

	// First pass: identify lerp tensors
	for _, src := range sources {
		if block, kind, ok := parseLerpTensor(src.name); ok {
			lerpMap[lerpKey{block, kind}] = src
			lerpBlocks[block]++
		}
	}

	for i, src := range sources {
		hfName := src.name

		// Skip individual lerp tensors; they'll be fused
		if _, _, ok := parseLerpTensor(hfName); ok {
			continue
		}

		ggmlType, err := stDTypeToGGML(src.dtype)
		if err != nil {
			return fmt.Errorf("tensor %q: %w", hfName, err)
		}

		dims := reverseShape(src.shape)
		ggufName := nameMapper(hfName)
		outputType := chooseOutputType(ggufName, ggmlType, len(src.shape))

		srcCopy := src
		capturedGGMLType := ggmlType
		capturedOutputType := outputType
		capturedHFName := hfName
		idx := i

		needsTranspose := isRWKVTransposeTensor(capturedHFName)
		needsPermute := strings.Contains(capturedHFName, "time_mix_w2")
		needsSqueeze := strings.Contains(capturedHFName, "time_mix_decay") && !strings.Contains(capturedHFName, "_w")

		// Rescale output/value weights
		var rescaleFactor float32
		if rescale > 0 {
			if strings.Contains(capturedHFName, "time_mix_output") || strings.Contains(capturedHFName, "channel_mix_value") {
				blockID := parseBlockID(capturedHFName)
				if blockID >= 0 {
					rescaleFactor = float32(math.Pow(2, float64(blockID/rescale)))
				}
			}
		}

		if needsTranspose && len(dims) == 2 {
			dims[0], dims[1] = dims[1], dims[0]
		}
		if needsPermute && len(dims) == 3 {
			dims[1], dims[2] = dims[2], dims[1]
		}
		if needsSqueeze {
			var squeezed []uint64
			for _, d := range dims {
				if d != 1 {
					squeezed = append(squeezed, d)
				}
			}
			if len(squeezed) > 0 {
				dims = squeezed
			}
		}

		capturedNeedsTranspose := needsTranspose
		capturedNeedsPermute := needsPermute
		capturedDims := dims
		capturedRescale := rescaleFactor

		getData := func() ([]byte, error) {
			progress("Converting tensor", idx+1, total)
			data, err := srcCopy.readData()
			if err != nil {
				return nil, err
			}
			data = convertDtype(data, capturedGGMLType, capturedOutputType)

			elemSize := int(capturedOutputType.ElementSize())
			if elemSize == 0 {
				elemSize = 4
			}

			if capturedNeedsTranspose && len(capturedDims) == 2 {
				// dims are already swapped; transpose the data layout
				cols := int(capturedDims[0])
				rows := int(capturedDims[1])
				data = transpose2D(data, cols, rows, elemSize)
			}
			if capturedNeedsPermute && len(capturedDims) == 3 {
				d0 := int(capturedDims[2])
				d1 := int(capturedDims[0])
				d2 := int(capturedDims[1])
				data = permute021(data, d0, d2, d1, elemSize)
			}
			if capturedRescale > 1 {
				invScale := 1.0 / capturedRescale
				data = transformFloats(data, capturedOutputType, func(v float32) float32 {
					return v * invScale
				})
			}
			return data, nil
		}

		w.addTensor(ggufName, dims, outputType, getData)
	}

	// Emit fused lerp tensors: stack (w, k, v, r, g) → shape (5, 1, 1, hidden)
	var blockIDs []int
	for bid := range lerpBlocks {
		blockIDs = append(blockIDs, bid)
	}
	sort.Ints(blockIDs)

	for _, bid := range blockIDs {
		kinds := []string{"w", "k", "v", "r", "g"}
		var lerpSources []tensorSource
		allPresent := true
		for _, k := range kinds {
			if s, ok := lerpMap[lerpKey{bid, k}]; ok {
				lerpSources = append(lerpSources, s)
			} else {
				allPresent = false
				break
			}
		}
		if !allPresent || len(lerpSources) == 0 {
			continue
		}

		firstType, _ := stDTypeToGGML(lerpSources[0].dtype)
		outputType := chooseOutputType("lerp", firstType, 1)
		fusedName := fmt.Sprintf("blk.%d.time_mix_lerp_fused.weight", bid)

		hidden := uint64(cfg.HiddenSize)
		fusedDims := []uint64{hidden, 1, 1, 5}

		capturedSources := lerpSources
		capturedFT := firstType
		capturedOT := outputType

		getData := func() ([]byte, error) {
			progress("Fusing lerp tensors", bid+1, len(blockIDs))
			elemSize := int(capturedOT.ElementSize())
			if elemSize == 0 {
				elemSize = 4
			}
			perTensor := int(hidden) * elemSize
			fused := make([]byte, 5*perTensor)
			for j, s := range capturedSources {
				data, err := s.readData()
				if err != nil {
					return nil, fmt.Errorf("reading lerp tensor: %w", err)
				}
				srcType, _ := stDTypeToGGML(s.dtype)
				data = convertDtype(data, srcType, capturedOT)
				// Squeeze to 1D
				if len(data) < perTensor {
					padded := make([]byte, perTensor)
					copy(padded, data)
					data = padded
				}
				copy(fused[j*perTensor:(j+1)*perTensor], data[:perTensor])
			}
			_ = capturedFT
			return fused, nil
		}

		w.addTensor(fusedName, fusedDims, outputType, getData)
	}

	return nil
}

func rwkv6NameMapper() func(string) string {
	r := strings.NewReplacer(
		"rwkv.embeddings", "token_embd",
		"rwkv.blocks.", "blk.",
		"rwkv.ln_out", "output_norm",
		"rwkv.head", "output",
		"model.embeddings", "token_embd",
		"model.blocks.", "blk.",
		"model.ln_out", "output_norm",
		"model.head", "output",
		".pre_ln", ".attn_norm",
		".ln1", ".attn_norm",
		".ln2", ".ffn_norm",
		".attention.", ".time_mix_",
		".feed_forward.key", ".channel_mix_key",
		".feed_forward.receptance", ".channel_mix_receptance",
		".feed_forward.value", ".channel_mix_value",
	)
	return func(hfName string) string {
		out := r.Replace(hfName)
		if !strings.HasSuffix(out, ".weight") && !strings.HasSuffix(out, ".bias") {
			out += ".weight"
		}
		return out
	}
}

func isRWKVTransposeTensor(name string) bool {
	return strings.Contains(name, "time_mix_w1") ||
		strings.Contains(name, "time_mix_decay_w1") ||
		strings.Contains(name, "time_mix_decay_w2")
}

// parseLerpTensor checks if a tensor is an individual lerp component.
// Returns (block_id, kind, true) for time_mix_lerp_{w,k,v,r,g} (NOT lerp_x).
func parseLerpTensor(name string) (int, string, bool) {
	for _, kind := range []string{"_w", "_k", "_v", "_r", "_g"} {
		suffix := "time_mix_lerp" + kind
		if strings.Contains(name, suffix) && !strings.Contains(name, "lerp_x") {
			return parseBlockID(name), kind[1:], true
		}
	}
	return 0, "", false
}

func parseBlockID(name string) int {
	// Extract block ID from patterns like "blk.N." or "blocks.N." or "layers.N."
	for _, prefix := range []string{"blk.", "blocks.", "layers."} {
		idx := strings.Index(name, prefix)
		if idx < 0 {
			continue
		}
		rest := name[idx+len(prefix):]
		blockID := 0
		for _, ch := range rest {
			if ch >= '0' && ch <= '9' {
				blockID = blockID*10 + int(ch-'0')
			} else {
				break
			}
		}
		return blockID
	}
	return -1
}

// --- RWKV7 ---

type rwkv7Converter struct {
	arch string
	cfg  *modelConfig
}

func (c *rwkv7Converter) Arch() string { return c.arch }

func (c *rwkv7Converter) WriteKV(w *ggufWriter, cfg *modelConfig) {
	prefix := c.arch
	w.addKV("general.architecture", prefix)
	w.addKV("general.name", cfg.ModelType)
	w.addKV("general.file_type", uint32(1))

	w.addKV(prefix+".block_count", uint32(cfg.NumHiddenLayers))
	w.addKV(prefix+".context_length", uint32(1<<20))
	w.addKV(prefix+".embedding_length", uint32(cfg.HiddenSize))
	w.addKV(prefix+".attention.head_count", uint32(0))
	w.addKV(prefix+".attention.layer_norm_rms_epsilon", float32(cfg.RmsNormEps))

	ffn := cfg.IntermediateSize
	if ffn == 0 {
		ffn = int(float64(cfg.HiddenSize)*3.5) / 32 * 32
	}
	w.addKV(prefix+".feed_forward_length", uint32(ffn))

	headSize := cfg.WkvHeadSize
	if headSize == 0 {
		headSize = 64
	}
	w.addKV(prefix+".wkv_head_size", uint32(headSize))
}

func (c *rwkv7Converter) ConvertTensors(w *ggufWriter, sources []tensorSource, cfg *modelConfig, progress ProgressFunc) error {
	// RWKV7 uses similar patterns to RWKV6 with some differences.
	// Re-use the same conversion logic with RWKV7-specific name mapping.
	sources = filterAndTieEmbeddings(sources, cfg)
	nameMapper := rwkv7NameMapper()
	total := len(sources)

	for i, src := range sources {
		hfName := src.name
		ggmlType, err := stDTypeToGGML(src.dtype)
		if err != nil {
			return fmt.Errorf("tensor %q: %w", hfName, err)
		}

		dims := reverseShape(src.shape)
		ggufName := nameMapper(hfName)
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

func rwkv7NameMapper() func(string) string {
	r := strings.NewReplacer(
		"rwkv.embeddings", "token_embd",
		"rwkv.blocks.", "blk.",
		"rwkv.ln_out", "output_norm",
		"rwkv.head", "output",
		"model.embeddings", "token_embd",
		"model.blocks.", "blk.",
		"model.ln_out", "output_norm",
		"model.head", "output",
		".pre_ln", ".attn_norm",
		".ln1", ".attn_norm",
		".ln2", ".ffn_norm",
		".attention.", ".time_mix_",
		".feed_forward.key", ".channel_mix_key",
		".feed_forward.receptance", ".channel_mix_receptance",
		".feed_forward.value", ".channel_mix_value",
	)
	return func(hfName string) string {
		out := r.Replace(hfName)
		if !strings.HasSuffix(out, ".weight") && !strings.HasSuffix(out, ".bias") {
			out += ".weight"
		}
		return out
	}
}

// transpose2D transposes a row-major 2D tensor: (rows, cols) → (cols, rows).
func transpose2D(data []byte, rows, cols, elemSize int) []byte {
	out := make([]byte, len(data))
	for i := 0; i < rows; i++ {
		for j := 0; j < cols; j++ {
			srcOff := (i*cols + j) * elemSize
			dstOff := (j*rows + i) * elemSize
			copy(out[dstOff:dstOff+elemSize], data[srcOff:srcOff+elemSize])
		}
	}
	return out
}

// permute021 reorders a 3D tensor from (a, b, c) to (a, c, b).
func permute021(data []byte, a, b, c, elemSize int) []byte {
	out := make([]byte, len(data))
	for i := 0; i < a; i++ {
		for j := 0; j < b; j++ {
			for k := 0; k < c; k++ {
				srcOff := (i*b*c + j*c + k) * elemSize
				dstOff := (i*c*b + k*b + j) * elemSize
				copy(out[dstOff:dstOff+elemSize], data[srcOff:srcOff+elemSize])
			}
		}
	}
	return out
}

// float32ToFloat16 is an alias exposed for RWKV transforms.
func float32ToFloat16Bits(f float32) uint16 {
	return float32BitsToFloat16(math.Float32bits(f))
}

func rwkvRescale(data []byte, dtype GGMLType, factor float32) []byte {
	if factor <= 1 {
		return data
	}
	invScale := 1.0 / factor
	switch dtype {
	case GGMLTypeF32:
		out := make([]byte, len(data))
		for i := 0; i+3 < len(data); i += 4 {
			v := math.Float32frombits(binary.LittleEndian.Uint32(data[i:]))
			binary.LittleEndian.PutUint32(out[i:], math.Float32bits(v*invScale))
		}
		return out
	case GGMLTypeF16:
		out := make([]byte, len(data))
		for i := 0; i+1 < len(data); i += 2 {
			v := float16ToFloat32(binary.LittleEndian.Uint16(data[i:]))
			binary.LittleEndian.PutUint16(out[i:], float32ToFloat16Bits(v*invScale))
		}
		return out
	}
	return data
}
