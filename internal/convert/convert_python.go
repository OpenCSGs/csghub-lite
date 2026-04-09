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
	"time"
)

// CSGHUB_LITE_CONVERTER_URL, if set, is the raw URL of convert_hf_to_gguf.py to download
// once per URL (e.g. GitLab mirror). When unset, the copy embedded in the binary is used
// (no GitHub access required at runtime).

const pythonDepsInstallArgs = "torch safetensors gguf transformers"

type converterRepairResult struct {
	attempted bool
	succeeded bool
	note      string
}

func pythonInstallHint() string {
	switch runtime.GOOS {
	case "darwin":
		return "  brew install python"
	case "windows":
		return "  winget install -e --id Python.Python.3.12\n" +
			"  If `winget` is unavailable, download Python from https://python.org and enable `Add Python to PATH` during setup."
	default:
		return "  sudo apt update && sudo apt install -y python3 python3-pip    # Debian / Ubuntu\n" +
			"  sudo dnf install -y python3 python3-pip                           # Fedora / RHEL / Rocky"
	}
}

func pythonDepsInstallHint() string {
	return fmt.Sprintf(
		"  pip3 install %s\n"+
			"  If `pip3` is unavailable, try:\n"+
			"    python3 -m pip install %s\n"+
			"    py -m pip install %s (Windows)",
		pythonDepsInstallArgs,
		pythonDepsInstallArgs,
		pythonDepsInstallArgs,
	)
}

func converterCacheDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".csghub-lite", "tools")
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
	required := []string{"torch", "safetensors", "gguf", "transformers"}
	var missing []string
	for _, pkg := range required {
		cmd := exec.Command(python, "-c", "import "+pkg)
		if cmd.Run() != nil {
			missing = append(missing, pkg)
		}
	}
	return strings.Join(missing, ", ")
}

func ensureConverterScript() (string, error) {
	if u := strings.TrimSpace(os.Getenv("CSGHUB_LITE_CONVERTER_URL")); u != "" {
		return ensureRemoteConverterScript(u)
	}
	return materializeBundledConverter()
}

func bundledConverterStamp() string {
	return fmt.Sprintf("%d %s", bundledConverterRevision, BundledConverterLLamacppRef)
}

func materializeBundledConverter() (string, error) {
	if len(bundledConverterPy) == 0 {
		return "", fmt.Errorf("embedded convert_hf_to_gguf.py is missing (rebuild csghub-lite)")
	}
	dir := converterCacheDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("creating tools dir: %w", err)
	}
	revPath := filepath.Join(dir, "bundled_convert_hf_revision")
	dst := filepath.Join(dir, "convert_hf_to_gguf_bundled.py")
	wantStamp := bundledConverterStamp()
	if prev, err := os.ReadFile(revPath); err == nil && string(prev) == wantStamp {
		if _, err := os.Stat(dst); err == nil {
			return dst, nil
		}
	}
	tmp := dst + ".tmp"
	if err := os.WriteFile(tmp, bundledConverterPy, 0o644); err != nil {
		return "", fmt.Errorf("writing converter: %w", err)
	}
	if err := os.Rename(tmp, dst); err != nil {
		os.Remove(tmp)
		return "", fmt.Errorf("installing converter: %w", err)
	}
	if err := os.WriteFile(revPath, []byte(wantStamp), 0o644); err != nil {
		return "", fmt.Errorf("writing converter revision: %w", err)
	}
	return dst, nil
}

func ensureRemoteConverterScript(rawURL string) (string, error) {
	dir := converterCacheDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("creating tools dir: %w", err)
	}
	urlPath := filepath.Join(dir, "remote_convert_hf_url")
	dst := filepath.Join(dir, "convert_hf_to_gguf_remote.py")
	if prev, err := os.ReadFile(urlPath); err == nil && string(prev) == rawURL {
		if _, err := os.Stat(dst); err == nil {
			return dst, nil
		}
	}
	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Get(rawURL)
	if err != nil {
		return "", fmt.Errorf("downloading converter from CSGHUB_LITE_CONVERTER_URL: %w", err)
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
		return "", fmt.Errorf("writing downloaded converter: %w", err)
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return "", err
	}
	if err := os.Rename(tmp, dst); err != nil {
		os.Remove(tmp)
		return "", err
	}
	if err := os.WriteFile(urlPath, []byte(rawURL), 0o644); err != nil {
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
		return "", fmt.Errorf(
			"this checkpoint is SafeTensors-only; csghub-lite converts it to GGUF once using the official llama.cpp Python script.\n"+
				"The Python runtime and conversion packages are not bundled with the release binary.\n\n"+
				"python3 was not found on PATH.\n"+
				"Please complete these one-time setup steps:\n"+
				"  1. Install Python 3 and make sure python3 / pip3 are available on PATH.\n"+
				"%s\n"+
				"  2. Install conversion deps:\n"+
				"%s\n\n"+
				"If the hub offers a GGUF build of the same model, download that instead to skip conversion.",
			pythonInstallHint(),
			pythonDepsInstallHint(),
		)
	}
	if missingDeps != "" {
		return "", fmt.Errorf(
			"this checkpoint is SafeTensors-only; csghub-lite converts it to GGUF once using the official llama.cpp Python script.\n"+
				"Those Python packages are not bundled with the release binary.\n\n"+
				"Missing: %s\n\n"+
				"Install (one-time):\n"+
				"%s\n\n"+
				"If a GGUF variant exists on CSGHub or Hugging Face, use it to skip conversion.",
			missingDeps,
			pythonDepsInstallHint(),
		)
	}

	step := "Preparing converter (bundled)"
	if strings.TrimSpace(os.Getenv("CSGHUB_LITE_CONVERTER_URL")) != "" {
		step = "Downloading converter"
	}
	progress(step, 0, 0)
	script, err := ensureConverterScript()
	if err != nil {
		return "", err
	}

	outputName := generateOutputName(modelDir, nil)
	outputPath := filepath.Join(modelDir, outputName)

	progress("Converting with official converter", 0, 0)
	if err := convertModelWithAutoRepair(python, script, modelDir, outputPath, progress); err != nil {
		return "", err
	}

	if _, err := os.Stat(outputPath); err != nil {
		return "", fmt.Errorf("converter finished but output file not found: %s", outputPath)
	}

	if hasVisionConfig(modelDir) {
		progress("Converting vision encoder (mmproj)", 0, 0)
		mmOut, mmErr := runMMProjConverter(python, script, modelDir)
		if mmErr != nil {
			log.Printf("mmproj conversion failed (non-fatal): %s\n%s", mmErr, lastNLines(mmOut, 5))
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

func convertModelWithAutoRepair(python, script, modelDir, outputPath string, progress ProgressFunc) error {
	output, err := runModelConverter(python, script, modelDir, outputPath)
	if err == nil {
		return nil
	}

	repair := attemptConverterAutoRepair(python, output, progress)
	if repair.attempted && repair.succeeded {
		_ = os.Remove(outputPath)
		progress("Retrying converter after Python package upgrade", 0, 0)
		output, err = runModelConverter(python, script, modelDir, outputPath)
		if err == nil {
			return nil
		}
	}

	_ = os.Remove(outputPath)
	return formatConverterFailure(err, output, repair.note)
}

func runModelConverter(python, script, modelDir, outputPath string) (string, error) {
	cmd := exec.Command(python, script, modelDir,
		"--outfile", outputPath,
		"--outtype", "f16",
	)
	output, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(output)), err
}

func runMMProjConverter(python, script, modelDir string) (string, error) {
	cmd := exec.Command(python, script, modelDir,
		"--outtype", "f16",
		"--mmproj",
	)
	cmd.Dir = modelDir
	output, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(output)), err
}

func attemptConverterAutoRepair(python, combined string, progress ProgressFunc) converterRepairResult {
	packages := packagesToAutoUpgradeForConverterFailure(combined)
	if len(packages) == 0 {
		return converterRepairResult{}
	}

	progress(fmt.Sprintf("Upgrading Python package%s: %s", pluralSuffix(len(packages)), strings.Join(packages, ", ")), 0, 0)
	pipOutput, pipErr := upgradePythonPackages(python, packages)
	command := fmt.Sprintf("%s -m pip install --upgrade %s", python, strings.Join(packages, " "))

	if pipErr != nil {
		pipSummary := lastNLines(pipOutput, 10)
		if pipSummary == "" {
			pipSummary = "(no pip output)"
		}
		return converterRepairResult{
			attempted: true,
			note: fmt.Sprintf(
				"\n\ncsghub-lite detected a Python package version mismatch and tried to run:\n  %s\n\n"+
					"Automatic upgrade failed: %s\n%s",
				command,
				pipErr,
				pipSummary,
			),
		}
	}

	return converterRepairResult{
		attempted: true,
		succeeded: true,
		note: fmt.Sprintf(
			"\n\ncsghub-lite auto-upgraded %s and retried once.",
			strings.Join(packages, ", "),
		),
	}
}

func upgradePythonPackages(python string, packages []string) (string, error) {
	args := []string{"-m", "pip", "install", "--upgrade"}
	args = append(args, packages...)
	cmd := exec.Command(python, args...)
	cmd.Env = append(os.Environ(), "PIP_DISABLE_PIP_VERSION_CHECK=1")
	output, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(output)), err
}

func packagesToAutoUpgradeForConverterFailure(combined string) []string {
	if combined == "" {
		return nil
	}

	var packages []string
	add := func(pkg string) {
		for _, existing := range packages {
			if existing == pkg {
				return
			}
		}
		packages = append(packages, pkg)
	}

	lower := strings.ToLower(combined)
	if strings.Contains(combined, "AttributeError") &&
		(strings.Contains(combined, "MODEL_ARCH") || strings.Contains(combined, "gguf.")) {
		add("gguf")
	}
	if strings.Contains(combined, "Transformers does not recognize this architecture") ||
		strings.Contains(combined, "pip install --upgrade transformers") ||
		strings.Contains(combined, "pip install git+https://github.com/huggingface/transformers.git") ||
		strings.Contains(lower, "no module named 'transformers.models.") {
		add("transformers")
	}

	return packages
}

func lastNLines(s string, n int) string {
	lines := strings.Split(s, "\n")
	if len(lines) <= n {
		return s
	}
	return strings.Join(lines[len(lines)-n:], "\n")
}

func formatConverterFailure(err error, output string, repairNote string) error {
	return fmt.Errorf(
		"convert_hf_to_gguf.py failed: %s\n%s%s%s",
		err,
		lastNLines(output, 5),
		repairNote,
		hintForConverterScriptFailure(output),
	)
}

func pluralSuffix(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

func hintForConverterScriptFailure(combined string) string {
	if combined == "" {
		return ""
	}
	// Typical mismatch: script from a new llama.cpp tag + older PyPI/distro `gguf`.
	if strings.Contains(combined, "AttributeError") &&
		(strings.Contains(combined, "MODEL_ARCH") || strings.Contains(combined, "gguf.")) {
		return fmt.Sprintf(
			"\n\nLikely the `gguf` Python package is older than this converter script expects.\n"+
				"If the automatic upgrade did not fix it, run: pip3 install -U gguf\n"+
				"Or point CSGHUB_LITE_CONVERTER_URL at a raw copy of a newer convert_hf_to_gguf.py (mirror).\n"+
				"To reset the bundled copy, delete convert_hf_to_gguf_bundled.py and bundled_convert_hf_revision under %s\n",
			converterCacheDir(),
		)
	}
	if strings.Contains(combined, "Transformers does not recognize this architecture") ||
		strings.Contains(combined, "pip install --upgrade transformers") ||
		strings.Contains(combined, "pip install git+https://github.com/huggingface/transformers.git") {
		return "\n\nThe installed `transformers` package looks too old for this model.\n" +
			"If the automatic upgrade did not fix it, run: pip3 install -U transformers\n"
	}
	return ""
}
