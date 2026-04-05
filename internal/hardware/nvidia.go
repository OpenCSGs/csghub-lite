package hardware

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

func ResolveNVIDIASMI() (string, error) {
	if path, err := exec.LookPath("nvidia-smi"); err == nil {
		return path, nil
	}

	return firstExistingPath(nvidiaSMICandidates(runtime.GOOS, os.Getenv))
}

func nvidiaSMICandidates(goos string, getenv func(string) string) []string {
	candidates := []string{
		"/usr/bin/nvidia-smi",
		"/usr/local/nvidia/bin/nvidia-smi",
		"/opt/nvidia/bin/nvidia-smi",
	}
	if goos == "windows" {
		if programFiles := getenv("ProgramFiles"); programFiles != "" {
			candidates = append(candidates, filepath.Join(programFiles, "NVIDIA Corporation", "NVSMI", "nvidia-smi.exe"))
		}
		if programFilesX86 := getenv("ProgramFiles(x86)"); programFilesX86 != "" {
			candidates = append(candidates, filepath.Join(programFilesX86, "NVIDIA Corporation", "NVSMI", "nvidia-smi.exe"))
		}
	}
	return candidates
}

func firstExistingPath(candidates []string) (string, error) {
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}
	return "", exec.ErrNotFound
}
