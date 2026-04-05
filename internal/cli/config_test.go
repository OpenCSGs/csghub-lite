package cli

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/opencsgs/csghub-lite/internal/config"
)

func TestRunConfigSetStorageDir(t *testing.T) {
	home := setupCLIConfigHome(t)
	root := filepath.Join(home, "shared-storage")

	if err := runConfigSet(nil, []string{"storage_dir", root}); err != nil {
		t.Fatalf("runConfigSet(storage_dir) error: %v", err)
	}

	config.Reset()
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load() error: %v", err)
	}

	wantModelDir := filepath.Join(root, config.ModelsDir)
	wantDatasetDir := filepath.Join(root, config.DatasetsDir)
	if cfg.ModelDir != wantModelDir {
		t.Fatalf("ModelDir = %q, want %q", cfg.ModelDir, wantModelDir)
	}
	if cfg.DatasetDir != wantDatasetDir {
		t.Fatalf("DatasetDir = %q, want %q", cfg.DatasetDir, wantDatasetDir)
	}
	if _, err := os.Stat(wantModelDir); err != nil {
		t.Fatalf("model dir not created: %v", err)
	}
	if _, err := os.Stat(wantDatasetDir); err != nil {
		t.Fatalf("dataset dir not created: %v", err)
	}
}

func TestRunConfigShowAndGetIncludeStorageDir(t *testing.T) {
	home := setupCLIConfigHome(t)
	root := filepath.Join(home, "shared-storage")

	if err := runConfigSet(nil, []string{"storage_dir", root}); err != nil {
		t.Fatalf("runConfigSet(storage_dir) error: %v", err)
	}

	showOutput := captureCLIStdout(t, func() {
		if err := runConfigShow(nil, nil); err != nil {
			t.Fatalf("runConfigShow() error: %v", err)
		}
	})
	if !strings.Contains(showOutput, "storage_dir: "+root) {
		t.Fatalf("config show output missing storage_dir: %q", showOutput)
	}
	if !strings.Contains(showOutput, "dataset_dir: "+filepath.Join(root, config.DatasetsDir)) {
		t.Fatalf("config show output missing dataset_dir: %q", showOutput)
	}

	getOutput := captureCLIStdout(t, func() {
		if err := runConfigGet(nil, []string{"storage_dir"}); err != nil {
			t.Fatalf("runConfigGet(storage_dir) error: %v", err)
		}
	})
	if strings.TrimSpace(getOutput) != root {
		t.Fatalf("config get storage_dir = %q, want %q", strings.TrimSpace(getOutput), root)
	}
}

func setupCLIConfigHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	config.Reset()
	t.Cleanup(config.Reset)
	return home
}

func captureCLIStdout(t *testing.T, fn func()) string {
	t.Helper()

	oldStdout := os.Stdout
	readPipe, writePipe, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error: %v", err)
	}
	os.Stdout = writePipe

	defer func() {
		os.Stdout = oldStdout
	}()

	done := make(chan string, 1)
	go func() {
		data, _ := io.ReadAll(readPipe)
		done <- string(data)
	}()

	fn()

	if err := writePipe.Close(); err != nil {
		t.Fatalf("writePipe.Close() error: %v", err)
	}
	return <-done
}
