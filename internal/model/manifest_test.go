package model

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSaveAndLoadManifest(t *testing.T) {
	dir := t.TempDir()

	original := &LocalModel{
		Namespace:    "OpenCSG",
		Name:         "test-model",
		Format:       FormatGGUF,
		Size:         1024 * 1024 * 100,
		Files:        []string{"model.gguf", "config.json"},
		DownloadedAt: time.Now().Truncate(time.Second),
		Description:  "A test model",
		License:      "MIT",
	}

	if err := SaveManifest(dir, original); err != nil {
		t.Fatalf("SaveManifest error: %v", err)
	}

	// Verify file exists
	mpath := ManifestPath(dir, "OpenCSG", "test-model")
	if _, err := os.Stat(mpath); os.IsNotExist(err) {
		t.Fatal("manifest file was not created")
	}

	loaded, err := LoadManifest(dir, "OpenCSG", "test-model")
	if err != nil {
		t.Fatalf("LoadManifest error: %v", err)
	}

	if loaded.Namespace != original.Namespace {
		t.Errorf("Namespace = %q, want %q", loaded.Namespace, original.Namespace)
	}
	if loaded.Name != original.Name {
		t.Errorf("Name = %q, want %q", loaded.Name, original.Name)
	}
	if loaded.Format != original.Format {
		t.Errorf("Format = %q, want %q", loaded.Format, original.Format)
	}
	if loaded.Size != original.Size {
		t.Errorf("Size = %d, want %d", loaded.Size, original.Size)
	}
	if len(loaded.Files) != len(original.Files) {
		t.Errorf("Files len = %d, want %d", len(loaded.Files), len(original.Files))
	}
	if loaded.Description != original.Description {
		t.Errorf("Description = %q, want %q", loaded.Description, original.Description)
	}
	if loaded.License != original.License {
		t.Errorf("License = %q, want %q", loaded.License, original.License)
	}
}

func TestLoadManifest_NotFound(t *testing.T) {
	dir := t.TempDir()
	_, err := LoadManifest(dir, "nonexistent", "model")
	if err == nil {
		t.Error("expected error for non-existent manifest")
	}
}

func TestDetectFormat(t *testing.T) {
	tests := []struct {
		name  string
		files []string
		want  Format
	}{
		{
			name:  "GGUF files",
			files: []string{"model-q4.gguf", "config.json"},
			want:  FormatGGUF,
		},
		{
			name:  "SafeTensors files",
			files: []string{"model.safetensors", "config.json"},
			want:  FormatSafeTensors,
		},
		{
			name:  "GGUF preferred over SafeTensors",
			files: []string{"model.safetensors", "model.gguf"},
			want:  FormatGGUF,
		},
		{
			name:  "unknown format",
			files: []string{"config.json", "tokenizer.json"},
			want:  FormatUnknown,
		},
		{
			name:  "case insensitive",
			files: []string{"Model.GGUF"},
			want:  FormatGGUF,
		},
		{
			name:  "empty",
			files: nil,
			want:  FormatUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectFormat(tt.files)
			if got != tt.want {
				t.Errorf("DetectFormat(%v) = %q, want %q", tt.files, got, tt.want)
			}
		})
	}
}

func TestFindModelFile(t *testing.T) {
	dir := t.TempDir()

	// Create a GGUF file
	if err := os.WriteFile(filepath.Join(dir, "model.gguf"), []byte("gguf"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	path, format, err := FindModelFile(dir)
	if err != nil {
		t.Fatalf("FindModelFile error: %v", err)
	}
	if format != FormatGGUF {
		t.Errorf("format = %q, want %q", format, FormatGGUF)
	}
	if filepath.Base(path) != "model.gguf" {
		t.Errorf("path = %q, want model.gguf", path)
	}
}

func TestFindModelFile_SafeTensors(t *testing.T) {
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "model.safetensors"), []byte("st"), 0o644); err != nil {
		t.Fatal(err)
	}

	path, format, err := FindModelFile(dir)
	if err != nil {
		t.Fatalf("FindModelFile error: %v", err)
	}
	if format != FormatSafeTensors {
		t.Errorf("format = %q, want %q", format, FormatSafeTensors)
	}
	if filepath.Base(path) != "model.safetensors" {
		t.Errorf("path = %q, want model.safetensors", path)
	}
}

func TestFindModelFile_NotFound(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, _, err := FindModelFile(dir)
	if err == nil {
		t.Error("expected error when no model file found")
	}
}

func TestFindModelFile_PicksHighestPrecisionGGUF(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "low-Q4_0.gguf"), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "high-Q8_0.gguf"), []byte("b"), 0o644); err != nil {
		t.Fatal(err)
	}

	path, format, err := FindModelFile(dir)
	if err != nil {
		t.Fatalf("FindModelFile: %v", err)
	}
	if format != FormatGGUF {
		t.Errorf("format = %q", format)
	}
	if filepath.Base(path) != "high-Q8_0.gguf" {
		t.Errorf("path = %q, want high-Q8_0.gguf", path)
	}
}

func TestFindModelFile_NestedQuantFolders(t *testing.T) {
	dir := t.TempDir()
	q4 := filepath.Join(dir, "Q4_0")
	q8 := filepath.Join(dir, "Q8_0")
	if err := os.MkdirAll(q4, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(q8, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(q4, "model.gguf"), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(q8, "model.gguf"), []byte("b"), 0o644); err != nil {
		t.Fatal(err)
	}

	path, format, err := FindModelFile(dir)
	if err != nil {
		t.Fatalf("FindModelFile: %v", err)
	}
	if format != FormatGGUF {
		t.Errorf("format = %q", format)
	}
	want := filepath.Join(q8, "model.gguf")
	if path != want {
		t.Errorf("path = %q, want %q", path, want)
	}
}
