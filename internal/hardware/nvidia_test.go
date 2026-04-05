package hardware

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestNVIDIASMICandidatesWindowsIncludesDefaultInstallDirs(t *testing.T) {
	got := nvidiaSMICandidates("windows", func(key string) string {
		switch key {
		case "ProgramFiles":
			return `C:\Program Files`
		case "ProgramFiles(x86)":
			return `C:\Program Files (x86)`
		default:
			return ""
		}
	})

	want := []string{
		"/usr/bin/nvidia-smi",
		"/usr/local/nvidia/bin/nvidia-smi",
		"/opt/nvidia/bin/nvidia-smi",
		filepath.Join(`C:\Program Files`, "NVIDIA Corporation", "NVSMI", "nvidia-smi.exe"),
		filepath.Join(`C:\Program Files (x86)`, "NVIDIA Corporation", "NVSMI", "nvidia-smi.exe"),
	}

	if len(got) != len(want) {
		t.Fatalf("len(candidates) = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("candidate[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestFirstExistingPathReturnsFirstMatch(t *testing.T) {
	dir := t.TempDir()
	first := filepath.Join(dir, "first.exe")
	second := filepath.Join(dir, "second.exe")
	if err := os.WriteFile(second, []byte("ok"), 0o644); err != nil {
		t.Fatalf("WriteFile(second) error = %v", err)
	}
	if err := os.WriteFile(first, []byte("ok"), 0o644); err != nil {
		t.Fatalf("WriteFile(first) error = %v", err)
	}

	got, err := firstExistingPath([]string{first, second})
	if err != nil {
		t.Fatalf("firstExistingPath returned error: %v", err)
	}
	if got != first {
		t.Fatalf("firstExistingPath = %q, want %q", got, first)
	}
}

func TestFirstExistingPathReturnsNotFound(t *testing.T) {
	got, err := firstExistingPath([]string{filepath.Join(t.TempDir(), "missing.exe")})
	if err != exec.ErrNotFound {
		t.Fatalf("firstExistingPath error = %v, want %v", err, exec.ErrNotFound)
	}
	if got != "" {
		t.Fatalf("firstExistingPath path = %q, want empty", got)
	}
}
