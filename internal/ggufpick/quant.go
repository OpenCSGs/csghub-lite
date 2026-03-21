package ggufpick

import (
	"path/filepath"
	"regexp"
	"strings"
)

// quantRanks: higher value = higher numerical precision / less aggressive quantization.
// Unknown tokens return -1 from quantRankFromStem.
var quantRanks = map[string]int{
	"f32":     1000,
	"bf16":    990,
	"f16":     980,
	"fp16":    980,
	"q8_0":    920,
	"q8_1":    915,
	"q6_k":    880,
	"q5_k_m":  860,
	"q5_k_s":  855,
	"q5_k":    850,
	"q5_1":    840,
	"q5_0":    835,
	"q4_k_m":  800,
	"q4_k_s":  795,
	"q4_k":    790,
	"q4_1":    785,
	"q4_0":    780,
	"q3_k_l":  750,
	"q3_k_m":  745,
	"q3_k_s":  740,
	"q3_k_xl": 738,
	"q3_k":    735,
	"q2_k":    700,
	"tq2_0":   680,
	"tq1_0":   670,
	"iq4_nl":  650,
	"iq4_xs":  640,
	"iq3_m":   620,
	"iq3_s":   610,
	"iq3_xs":  600,
	"iq3_xxs": 590,
	"iq2_m":   570,
	"iq2_xs":  560,
	"iq2_xxs": 550,
	"iq1_m":   520,
	"iq1_s":   510,
}

var shardSuffixRe = regexp.MustCompile(`-\d+-of-\d+$`)

// IsMMProjGGUF reports whether name looks like a multimodal projector GGUF.
func IsMMProjGGUF(name string) bool {
	lower := strings.ToLower(name)
	return strings.HasSuffix(lower, ".gguf") && strings.Contains(lower, "mmproj")
}

// IsWeightGGUF is a non-mmproj .gguf file (main model weights).
func IsWeightGGUF(name string) bool {
	lower := strings.ToLower(filepath.Base(name))
	if !strings.HasSuffix(lower, ".gguf") {
		return false
	}
	return !strings.Contains(lower, "mmproj")
}

// QuantRank returns a precision rank for a weight GGUF basename; higher is better.
// Returns -1 if no known quantization token is found.
func QuantRank(basename string) int {
	stem := normalizeGGUFStem(filepath.Base(basename))
	if stem == "" {
		return -1
	}
	return quantRankFromStem(stem)
}

func normalizeGGUFStem(basename string) string {
	lower := strings.ToLower(basename)
	if !strings.HasSuffix(lower, ".gguf") {
		return ""
	}
	stem := basename[:len(basename)-len(".gguf")]
	stem = strings.ToLower(stem)
	stem = shardSuffixRe.ReplaceAllString(stem, "")
	return stem
}

func quantRankFromStem(stem string) int {
	tokens := strings.Split(stem, "-")
	if len(tokens) == 0 {
		return -1
	}
	// Try last 1..3 tokens joined with underscores (e.g. q8_0, q4_k_m).
	for n := 3; n >= 1; n-- {
		if len(tokens) < n {
			continue
		}
		cand := strings.Join(tokens[len(tokens)-n:], "_")
		if r, ok := quantRanks[cand]; ok {
			return r
		}
	}
	return -1
}

// FileEntry is a minimal file description for GGUF download filtering.
type FileEntry struct {
	Path string
	Name string
	Size int64
}

// FilterWeightGGUFFiles keeps every shard of the highest-known-precision variant.
// If there is at most one weight file, or no file has a known quant token, entries are returned unchanged.
func FilterWeightGGUFFiles(entries []FileEntry) []FileEntry {
	if len(entries) <= 1 {
		return entries
	}
	ranks := make([]int, len(entries))
	maxRank := -1
	known := false
	for i, e := range entries {
		base := e.Name
		if base == "" {
			base = filepath.Base(e.Path)
		}
		r := QuantRank(base)
		ranks[i] = r
		if r >= 0 {
			known = true
			if r > maxRank {
				maxRank = r
			}
		}
	}
	if !known {
		return entries
	}
	var out []FileEntry
	for i, e := range entries {
		if ranks[i] == maxRank {
			out = append(out, e)
		}
	}
	if len(out) == 0 {
		return entries
	}
	return out
}

// BestWeightGGUFName picks the highest-precision weight GGUF basename.
// Tie-breaker: lexicographic order on name for stability.
func BestWeightGGUFName(names []string) string {
	if len(names) == 0 {
		return ""
	}
	if len(names) == 1 {
		return names[0]
	}
	best := names[0]
	bestR := QuantRank(best)
	for _, n := range names[1:] {
		r := QuantRank(n)
		if r > bestR || (r == bestR && n < best) {
			best = n
			bestR = r
		}
	}
	return best
}
