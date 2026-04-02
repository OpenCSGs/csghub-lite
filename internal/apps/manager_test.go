package apps

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAppSpecsRequirePTYForClaudeInstall(t *testing.T) {
	var claude appSpec
	found := false
	for _, spec := range appSpecs() {
		if spec.id == "claude-code" {
			claude = spec
			found = true
			break
		}
	}
	if !found {
		t.Fatal("claude-code spec not found")
	}
	if claude.unix == nil || !claude.unix.requiresPTY {
		t.Fatal("claude-code unix installer should require PTY")
	}
	if claude.windows == nil || !claude.windows.requiresPTY {
		t.Fatal("claude-code windows installer should require PTY")
	}
	if claude.uninstallUnix != nil && claude.uninstallUnix.requiresPTY {
		t.Fatal("claude-code unix uninstaller should not require PTY")
	}
	if claude.uninstallWin != nil && claude.uninstallWin.requiresPTY {
		t.Fatal("claude-code windows uninstaller should not require PTY")
	}
}

func TestDetectInstalledBinaryPathFallsBackToCommonDirs(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("PATH", "")

	binDir := filepath.Join(homeDir, ".local", "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("mkdir bin dir: %v", err)
	}
	binaryPath := filepath.Join(binDir, "claude")
	if err := os.WriteFile(binaryPath, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("write binary: %v", err)
	}

	got, ok := detectInstalledBinaryPath("claude")
	if !ok {
		t.Fatal("detectInstalledBinaryPath should find claude in ~/.local/bin")
	}
	if got != binaryPath {
		t.Fatalf("detectInstalledBinaryPath = %q, want %q", got, binaryPath)
	}
}

func TestSummarizeFailureLogsPrefersExplicitError(t *testing.T) {
	lines := []string{
		"2026-03-28 23:13:43 INFO: preparing uninstaller",
		"2026-03-28 23:13:44 npm error ENOTEMPTY: directory not empty, rename '/opt/homebrew/lib/node_modules/openclaw' -> '/opt/homebrew/lib/node_modules/.openclaw-2N5mgx4q'",
		"2026-03-28 23:13:45 ERROR: OpenClaw binary is still available at /Users/test/bin/openclaw",
	}

	got := summarizeFailureLogs(lines)
	want := "OpenClaw binary is still available at /Users/test/bin/openclaw"
	if got != want {
		t.Fatalf("summarizeFailureLogs = %q, want %q", got, want)
	}
}

func TestSummarizeFailureLogsReturnsActionableNPMError(t *testing.T) {
	lines := []string{
		"2026-03-28 23:13:44 npm error code ENOTEMPTY",
		"2026-03-28 23:13:44 npm error syscall rename",
		"2026-03-28 23:13:44 npm error path /opt/homebrew/lib/node_modules/openclaw",
		"2026-03-28 23:13:44 npm error dest /opt/homebrew/lib/node_modules/.openclaw-2N5mgx4q",
		"2026-03-28 23:13:44 npm error errno -66",
		"2026-03-28 23:13:44 npm error ENOTEMPTY: directory not empty, rename '/opt/homebrew/lib/node_modules/openclaw' -> '/opt/homebrew/lib/node_modules/.openclaw-2N5mgx4q'",
		"2026-03-28 23:13:44 npm error A complete log of this run can be found in: /Users/test/.npm/_logs/debug.log",
	}

	got := summarizeFailureLogs(lines)
	want := "npm error ENOTEMPTY: directory not empty, rename '/opt/homebrew/lib/node_modules/openclaw' -> '/opt/homebrew/lib/node_modules/.openclaw-2N5mgx4q'"
	if got != want {
		t.Fatalf("summarizeFailureLogs = %q, want %q", got, want)
	}
}

func TestStripLogTimestamp(t *testing.T) {
	got := stripLogTimestamp("2026-03-28 23:13:44 npm error ENOTEMPTY: directory not empty")
	want := "npm error ENOTEMPTY: directory not empty"
	if got != want {
		t.Fatalf("stripLogTimestamp = %q, want %q", got, want)
	}
}
