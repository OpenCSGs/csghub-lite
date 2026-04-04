package dataset

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureLocalDatasetFiles_BackfillsMissingEntries(t *testing.T) {
	baseDir := t.TempDir()
	datasetDir := DatasetDir(baseDir, "Acme", "demo")
	if err := os.MkdirAll(filepath.Join(datasetDir, "train"), 0o755); err != nil {
		t.Fatalf("mkdir train: %v", err)
	}
	if err := os.WriteFile(filepath.Join(datasetDir, "train", "data.jsonl"), []byte("demo"), 0o644); err != nil {
		t.Fatalf("write data file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(datasetDir, "README.md"), []byte("# demo"), 0o644); err != nil {
		t.Fatalf("write README: %v", err)
	}

	ld := &LocalDataset{
		Namespace: "Acme",
		Name:      "demo",
		Files:     []string{"data.jsonl", "README.md"},
	}
	if err := SaveManifest(baseDir, ld); err != nil {
		t.Fatalf("save legacy manifest: %v", err)
	}

	loaded, err := LoadManifest(baseDir, "Acme", "demo")
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}

	changed, err := EnsureLocalDatasetFiles(datasetDir, loaded)
	if err != nil {
		t.Fatalf("EnsureLocalDatasetFiles: %v", err)
	}
	if !changed {
		t.Fatal("changed = false, want true for legacy manifest backfill")
	}
	if len(loaded.FileEntries) != 2 {
		t.Fatalf("file_entries len = %d, want 2", len(loaded.FileEntries))
	}
	if got := loaded.Files; len(got) != 2 || got[0] != "README.md" || got[1] != "train/data.jsonl" {
		t.Fatalf("files = %#v, want sorted relative paths", got)
	}
	for _, entry := range loaded.FileEntries {
		if entry.Path == "" || entry.Size <= 0 || entry.SHA256 == "" {
			t.Fatalf("invalid file entry: %#v", entry)
		}
	}

	changed, err = EnsureLocalDatasetFiles(datasetDir, loaded)
	if err != nil {
		t.Fatalf("EnsureLocalDatasetFiles second pass: %v", err)
	}
	if changed {
		t.Fatal("changed = true on second pass, want false")
	}
}
