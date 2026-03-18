package convert

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const converterURL = "https://raw.githubusercontent.com/ggml-org/llama.cpp/master/convert_hf_to_gguf.py"

func converterCacheDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".csghub-lite", "tools")
}

func converterScriptPath() string {
	return filepath.Join(converterCacheDir(), "convert_hf_to_gguf.py")
}

// findPythonEnv locates a suitable Python interpreter.
// Returns (pythonPath, missingDeps) where:
//   - pythonPath != "" && missingDeps == "": ready to use
//   - pythonPath != "" && missingDeps != "": Python found but packages missing
//   - pythonPath == "": no Python found at all
func findPythonEnv() (pythonPath string, missingDeps string) {
	candidates := []string{"python3.13", "python3.12", "python3.11", "python3.10", "python3", "python"}
	if runtime.GOOS == "windows" {
		candidates = []string{"python", "python3"}
	}

	extraPaths := []string{
		"/opt/homebrew/bin/python3.13",
		"/opt/homebrew/bin/python3.12",
		"/opt/homebrew/bin/python3.11",
		"/opt/homebrew/bin/python3.10",
		"/opt/homebrew/bin/python3",
		"/usr/local/bin/python3",
	}

	var firstPython string

	for _, name := range candidates {
		if p, err := exec.LookPath(name); err == nil {
			if firstPython == "" {
				firstPython = p
			}
			if missing := checkPythonDeps(p); missing == "" {
				return p, ""
			}
		}
	}
	for _, p := range extraPaths {
		if _, err := os.Stat(p); err == nil {
			if firstPython == "" {
				firstPython = p
			}
			if missing := checkPythonDeps(p); missing == "" {
				return p, ""
			}
		}
	}

	if firstPython != "" {
		return firstPython, checkPythonDeps(firstPython)
	}
	return "", ""
}

// checkPythonDeps returns a comma-separated list of missing packages, or "" if all present.
func checkPythonDeps(python string) string {
	required := []string{"torch", "safetensors", "gguf"}
	var missing []string
	for _, pkg := range required {
		cmd := exec.Command(python, "-c", "import "+pkg)
		if cmd.Run() != nil {
			missing = append(missing, pkg)
		}
	}
	return strings.Join(missing, ", ")
}

func downloadConverter() (string, error) {
	dst := converterScriptPath()
	if _, err := os.Stat(dst); err == nil {
		return dst, nil
	}

	dir := converterCacheDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("creating tools dir: %w", err)
	}

	resp, err := http.Get(converterURL)
	if err != nil {
		return "", fmt.Errorf("downloading converter: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("downloading converter: HTTP %d", resp.StatusCode)
	}

	tmp := dst + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		os.Remove(tmp)
		return "", err
	}
	f.Close()

	if err := os.Rename(tmp, dst); err != nil {
		os.Remove(tmp)
		return "", err
	}
	return dst, nil
}

// ConvertPython uses the official llama.cpp convert_hf_to_gguf.py to convert
// a HuggingFace model directory to GGUF format. Returns the path to the
// generated GGUF file. Requires python3 with torch, safetensors, and gguf.
func ConvertPython(modelDir string, progress ProgressFunc) (string, error) {
	if progress == nil {
		progress = func(string, int, int) {}
	}

	python, missingDeps := findPythonEnv()
	if python == "" {
		installHint := "brew install python3 (macOS) / apt install python3 (Linux) / https://python.org (Windows)"
		return "", fmt.Errorf("this model requires Python 3 for auto-conversion, but python3 was not found.\n"+
			"Install Python: %s\n"+
			"Then run: pip3 install torch safetensors gguf", installHint)
	}
	if missingDeps != "" {
		return "", fmt.Errorf("this model requires Python packages for auto-conversion.\n"+
			"Missing packages: %s\n"+
			"Install with: pip3 install %s", missingDeps, missingDeps)
	}

	progress("Downloading converter", 0, 0)
	script, err := downloadConverter()
	if err != nil {
		return "", err
	}

	outputName := generateOutputName(modelDir, nil)
	outputPath := filepath.Join(modelDir, outputName)

	progress("Converting with official converter", 0, 0)

	cmd := exec.Command(python, script, modelDir,
		"--outfile", outputPath,
		"--outtype", "f16",
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		lines := strings.TrimSpace(string(output))
		lastLines := lastNLines(lines, 5)
		return "", fmt.Errorf("convert_hf_to_gguf.py failed: %s\n%s", err, lastLines)
	}

	if _, err := os.Stat(outputPath); err != nil {
		return "", fmt.Errorf("converter finished but output file not found: %s", outputPath)
	}

	if hasVisionConfig(modelDir) {
		progress("Converting vision encoder (mmproj)", 0, 0)
		mmCmd := exec.Command(python, script, modelDir,
			"--outtype", "f16",
			"--mmproj",
		)
		mmCmd.Dir = modelDir
		mmOut, mmErr := mmCmd.CombinedOutput()
		if mmErr != nil {
			log.Printf("mmproj conversion failed (non-fatal): %s\n%s", mmErr, lastNLines(strings.TrimSpace(string(mmOut)), 5))
		} else {
			log.Printf("mmproj conversion succeeded")
		}
	}

	return outputPath, nil
}

func hasVisionConfig(modelDir string) bool {
	data, err := os.ReadFile(filepath.Join(modelDir, "config.json"))
	if err != nil {
		return false
	}
	var cfg struct {
		VisionConfig json.RawMessage `json:"vision_config"`
	}
	if json.Unmarshal(data, &cfg) != nil {
		return false
	}
	return len(cfg.VisionConfig) > 0
}

// PythonConverterAvailable returns true if python3 and the required
// dependencies are available for running the official converter.
func PythonConverterAvailable() bool {
	p, missing := findPythonEnv()
	return p != "" && missing == ""
}

func lastNLines(s string, n int) string {
	lines := strings.Split(s, "\n")
	if len(lines) <= n {
		return s
	}
	return strings.Join(lines[len(lines)-n:], "\n")
}
