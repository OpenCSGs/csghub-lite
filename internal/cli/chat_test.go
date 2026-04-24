package cli

import "testing"

func TestNewChatCmdExposesLlamaRuntimeFlags(t *testing.T) {
	cmd := newChatCmd()

	for _, name := range []string{"system", "num-ctx", "num-parallel", "n-gpu-layers", "cache-type-k", "cache-type-v", "dtype"} {
		if f := cmd.Flags().Lookup(name); f == nil {
			t.Fatalf("expected --%s flag", name)
		}
	}
}
