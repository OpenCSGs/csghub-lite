package inference

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/opencsgs/csghub-lite/internal/model"
)

func TestLoadEngine_SafeTensorsAutoConvert(t *testing.T) {
	dir := t.TempDir()
	// SafeTensors without config.json should fail during conversion (missing config).
	os.WriteFile(filepath.Join(dir, "model.safetensors"), []byte("data"), 0o644)

	lm := &model.LocalModel{
		Namespace: "test",
		Name:      "model",
		Format:    model.FormatSafeTensors,
	}

	_, err := LoadEngine(dir, lm)
	if err == nil {
		t.Fatal("expected error for SafeTensors model without config.json")
	}
	// Should report auto-conversion failure (not ErrUnsupportedFormat).
	if strings.Contains(err.Error(), "auto-converting SafeTensors") {
		return // expected
	}
	t.Errorf("unexpected error: %v", err)
}

func TestLoadEngine_NoModelFile(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "config.json"), []byte("{}"), 0o644)

	lm := &model.LocalModel{
		Namespace: "test",
		Name:      "model",
		Format:    model.FormatUnknown,
	}

	_, err := LoadEngine(dir, lm)
	if err == nil {
		t.Fatal("expected error when no model file exists")
	}
}
