package convert

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLlamaCppVersionLockstepWithInstallScripts(t *testing.T) {
	t.Helper()

	repoRoot := filepath.Clean(filepath.Join("..", ".."))
	wantShell := `LLAMA_CPP_DEFAULT_TAG="${CSGHUB_LITE_LLAMA_CPP_TAG:-` + BundledConverterLLamacppRef + `}"`
	wantPowerShell := `$LlamaCppDefaultTag = if ($env:CSGHUB_LITE_LLAMA_CPP_TAG) { $env:CSGHUB_LITE_LLAMA_CPP_TAG } else { "` + BundledConverterLLamacppRef + `" }`

	cases := []struct {
		path string
		want string
	}{
		{path: filepath.Join(repoRoot, "scripts", "install.sh"), want: wantShell},
		{path: filepath.Join(repoRoot, "scripts", "install.ps1"), want: wantPowerShell},
	}

	for _, tc := range cases {
		data, err := os.ReadFile(tc.path)
		if err != nil {
			t.Fatalf("read %s: %v", tc.path, err)
		}
		if !strings.Contains(string(data), tc.want) {
			t.Fatalf("%s is not pinned to BundledConverterLLamacppRef=%s", tc.path, BundledConverterLLamacppRef)
		}
	}
}
