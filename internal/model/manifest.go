package model

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/opencsgs/csghub-lite/internal/ggufpick"
)

// Vision-related HuggingFace architecture suffixes/names.
var visionArchitectures = map[string]bool{
	"Qwen2VLForConditionalGeneration":    true,
	"Qwen2_5_VLForConditionalGeneration": true,
	"Qwen3_5ForConditionalGeneration":    true,
	"Qwen3_5MoeForConditionalGeneration": true,
	"LlavaForConditionalGeneration":      true,
	"LlavaNextForConditionalGeneration":  true,
	"CogVLMForCausalLM":                  true,
	"InternVLChatModel":                  true,
	"MiniCPMV":                           true,
	"Phi3VForCausalLM":                   true,
	"Gemma3ForConditionalGeneration":     true,
}

// DetectPipelineTag reads config.json in modelDir and returns "image-text-to-text"
// for vision models, "text-generation" for text-only models.
func DetectPipelineTag(modelDir string) string {
	data, err := os.ReadFile(filepath.Join(modelDir, "config.json"))
	if err != nil {
		return "text-generation"
	}
	var cfg struct {
		Architectures []string `json:"architectures"`
	}
	if json.Unmarshal(data, &cfg) != nil {
		return "text-generation"
	}
	for _, arch := range cfg.Architectures {
		if visionArchitectures[arch] {
			return "image-text-to-text"
		}
	}
	return "text-generation"
}

// FindMMProj looks for a multimodal projector GGUF file (mmproj) in the model directory.
func FindMMProj(modelDir string) string {
	entries, err := os.ReadDir(modelDir)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		lower := strings.ToLower(e.Name())
		if strings.Contains(lower, "mmproj") && strings.HasSuffix(lower, ".gguf") {
			return filepath.Join(modelDir, e.Name())
		}
	}
	return ""
}

// SaveManifest writes a model manifest to disk.
func SaveManifest(baseDir string, m *LocalModel) error {
	normalizeLocalModel(m)
	mpath := ManifestPath(baseDir, m.Namespace, m.Name)
	if err := os.MkdirAll(filepath.Dir(mpath), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(mpath, data, 0o644)
}

// LoadManifest reads a model manifest from disk.
func LoadManifest(baseDir, namespace, name string) (*LocalModel, error) {
	mpath := ManifestPath(baseDir, namespace, name)
	data, err := os.ReadFile(mpath)
	if err != nil {
		return nil, err
	}
	var m LocalModel
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	normalizeLocalModel(&m)
	return &m, nil
}

// DetectFormat guesses the model format from the file names.
func DetectFormat(files []string) Format {
	for _, f := range files {
		lower := strings.ToLower(f)
		if strings.HasSuffix(lower, ".gguf") {
			return FormatGGUF
		}
	}
	for _, f := range files {
		lower := strings.ToLower(f)
		if strings.HasSuffix(lower, ".safetensors") {
			return FormatSafeTensors
		}
	}
	return FormatUnknown
}

// FindModelFile returns the primary model file (GGUF or SafeTensors).
func FindModelFile(modelDir string) (string, Format, error) {
	entries, err := os.ReadDir(modelDir)
	if err != nil {
		return "", FormatUnknown, err
	}

	// Prefer GGUF weight files (skip multimodal projector); recurse into subdirs; pick highest precision.
	ggufRel, err := ggufpick.CollectWeightGGUFRelPaths(modelDir)
	if err != nil {
		return "", FormatUnknown, err
	}
	if len(ggufRel) > 0 {
		best := ggufpick.BestWeightGGUFRelPath(ggufRel)
		return filepath.Join(modelDir, best), FormatGGUF, nil
	}
	// Then SafeTensors
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(strings.ToLower(e.Name()), ".safetensors") {
			return filepath.Join(modelDir, e.Name()), FormatSafeTensors, nil
		}
	}
	return "", FormatUnknown, os.ErrNotExist
}
