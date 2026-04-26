package inference

import (
	"path/filepath"
	"runtime"
	"testing"
)

func TestLlamaBinaryCandidatePathsIncludesInstallerLocations(t *testing.T) {
	home := filepath.Join("Users", "james")
	exePath := filepath.Join(home, ".local", "bin", "csghub-lite")
	paths := llamaBinaryCandidatePaths(home, exePath, runtime.GOOS)

	wants := []string{
		filepath.Join(home, ".local", "bin", platformLlamaServerName()),
		filepath.Join(home, "bin", platformLlamaServerName()),
		filepath.Join(filepath.Dir(exePath), platformLlamaServerName()),
	}
	for _, want := range wants {
		if !containsPath(paths, want) {
			t.Fatalf("llamaBinaryCandidatePaths() missing %q in %#v", want, paths)
		}
	}
}

func platformLlamaServerName() string {
	if runtime.GOOS == "windows" {
		return "llama-server.exe"
	}
	return "llama-server"
}

func containsPath(paths []string, want string) bool {
	for _, path := range paths {
		if path == want {
			return true
		}
	}
	return false
}
