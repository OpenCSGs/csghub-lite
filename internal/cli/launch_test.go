package cli

import "testing"

func TestResolveLaunchTarget(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{input: "claude-code", want: "claude-code"},
		{input: "claude", want: "claude-code"},
		{input: "open-code", want: "open-code"},
		{input: "opencode", want: "open-code"},
		{input: "codex", want: "codex"},
		{input: "openclaw", want: "openclaw"},
		{input: "dify", want: "dify"},
		{input: "anythingllm", want: "anythingllm"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			target, err := resolveLaunchTarget(tt.input)
			if err != nil {
				t.Fatalf("resolveLaunchTarget(%q) error: %v", tt.input, err)
			}
			if target.AppID != tt.want {
				t.Fatalf("resolveLaunchTarget(%q) = %q, want %q", tt.input, target.AppID, tt.want)
			}
		})
	}
}

func TestResolveLaunchTargetUnknown(t *testing.T) {
	if _, err := resolveLaunchTarget("unknown-app"); err == nil {
		t.Fatal("resolveLaunchTarget(unknown-app) expected error")
	}
}
