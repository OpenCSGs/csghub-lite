package dataset

import (
	"encoding/json"
	"os"
	"path/filepath"
)

func SaveManifest(baseDir string, d *LocalDataset) error {
	normalizeLocalDataset(d)
	mpath := ManifestPath(baseDir, d.Namespace, d.Name)
	if err := os.MkdirAll(filepath.Dir(mpath), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(mpath, data, 0o644)
}

func LoadManifest(baseDir, namespace, name string) (*LocalDataset, error) {
	mpath := ManifestPath(baseDir, namespace, name)
	data, err := os.ReadFile(mpath)
	if err != nil {
		return nil, err
	}
	var d LocalDataset
	if err := json.Unmarshal(data, &d); err != nil {
		return nil, err
	}
	normalizeLocalDataset(&d)
	return &d, nil
}
