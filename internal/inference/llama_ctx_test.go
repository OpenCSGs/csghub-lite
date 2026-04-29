package inference

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveNumCtxUsesExplicitRequest(t *testing.T) {
	dir := t.TempDir()
	if got := ResolveNumCtx(dir, 12288); got != 12288 {
		t.Fatalf("ResolveNumCtx returned %d, want %d", got, 12288)
	}
}

func TestResolveNumCtxUsesEnvOverride(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CSGHUB_LITE_LLAMA_NUM_CTX", "24576")

	if got := ResolveNumCtx(dir, 0); got != 24576 {
		t.Fatalf("ResolveNumCtx returned %d, want %d", got, 24576)
	}
}

func TestResolveNumCtxExpandsFromModelConfig(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(`{"max_position_embeddings":40960}`), 0o644); err != nil {
		t.Fatalf("write config.json: %v", err)
	}

	if got := ResolveNumCtx(dir, 0); got != 16384 {
		t.Fatalf("ResolveNumCtx returned %d, want %d", got, 16384)
	}
}

func TestResolveNumCtxFallsBackToDefault(t *testing.T) {
	dir := t.TempDir()

	if got := ResolveNumCtx(dir, 0); got != 8192 {
		t.Fatalf("ResolveNumCtx returned %d, want %d", got, 8192)
	}
}

func TestResolveNumParallelFallsBackToSingleSlot(t *testing.T) {
	if got := ResolveNumParallel(0); got != 1 {
		t.Fatalf("ResolveNumParallel returned %d, want 1", got)
	}
}

func TestResolveNGPULayersUsesExplicitRequest(t *testing.T) {
	if got := ResolveNGPULayers(42); got != 42 {
		t.Fatalf("ResolveNGPULayers returned %d, want %d", got, 42)
	}
}

func TestNormalizeNGPULayersRejectsLessThanUnset(t *testing.T) {
	if _, err := NormalizeNGPULayers(-2); err == nil {
		t.Fatal("expected invalid n_gpu_layers error")
	}
}
