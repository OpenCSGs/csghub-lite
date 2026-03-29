package apps

import "testing"

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
