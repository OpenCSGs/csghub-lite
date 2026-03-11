package model

import (
	"os"
	"path/filepath"
)

// ModelDir returns the directory for a specific model.
func ModelDir(baseDir, namespace, name string) string {
	return filepath.Join(baseDir, namespace, name)
}

// ManifestPath returns the path to the manifest file for a model.
func ManifestPath(baseDir, namespace, name string) string {
	return filepath.Join(ModelDir(baseDir, namespace, name), "manifest.json")
}

// EnsureModelDir creates the directory for a model if it doesn't exist.
func EnsureModelDir(baseDir, namespace, name string) error {
	return os.MkdirAll(ModelDir(baseDir, namespace, name), 0o755)
}

// RemoveModelDir removes the directory for a model.
func RemoveModelDir(baseDir, namespace, name string) error {
	dir := ModelDir(baseDir, namespace, name)
	if err := os.RemoveAll(dir); err != nil {
		return err
	}
	// Clean up empty parent namespace directory
	nsDir := filepath.Join(baseDir, namespace)
	entries, err := os.ReadDir(nsDir)
	if err == nil && len(entries) == 0 {
		os.Remove(nsDir)
	}
	return nil
}

// ListNamespaces returns all namespace directories under the model base dir.
func ListNamespaces(baseDir string) ([]string, error) {
	entries, err := os.ReadDir(baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var namespaces []string
	for _, e := range entries {
		if e.IsDir() {
			namespaces = append(namespaces, e.Name())
		}
	}
	return namespaces, nil
}

// ListModelsInNamespace returns all model directories under a namespace.
func ListModelsInNamespace(baseDir, namespace string) ([]string, error) {
	nsDir := filepath.Join(baseDir, namespace)
	entries, err := os.ReadDir(nsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var models []string
	for _, e := range entries {
		if e.IsDir() {
			models = append(models, e.Name())
		}
	}
	return models, nil
}
