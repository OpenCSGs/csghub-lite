package model

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// SaveManifest writes a model manifest to disk.
func SaveManifest(baseDir string, m *LocalModel) error {
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

	// Prefer GGUF files
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(strings.ToLower(e.Name()), ".gguf") {
			return filepath.Join(modelDir, e.Name()), FormatGGUF, nil
		}
	}
	// Then SafeTensors
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(strings.ToLower(e.Name()), ".safetensors") {
			return filepath.Join(modelDir, e.Name()), FormatSafeTensors, nil
		}
	}
	return "", FormatUnknown, os.ErrNotExist
}
