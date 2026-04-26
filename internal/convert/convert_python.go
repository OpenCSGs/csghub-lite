package convert

import (
	"archive/tar"
	"compress/gzip"
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

const (
	pythonCPUOnlyTorchInstallArgs = "--index-url https://download.pytorch.org/whl/cpu torch"
	pythonDepsInstallArgs         = "safetensors transformers"
	regionCN                      = "CN"
	regionINTL                    = "INTL"
	llamaCppGitHubRepo            = "https://github.com/ggml-org/llama.cpp"
	llamaCppGiteeRepo             = "https://gitee.com/xzgan/llama.cpp"
)

type converterRepairResult struct {
	attempted bool
	succeeded bool
	note      string
}

type converterRepairPlan struct {
	installBundledGGUFPy bool
	upgradePackages      []string
}

type llamaCppSource struct {
	name       string
	repoURL    string
	archiveURL string
}

func pythonInstallHint() string {
	switch runtime.GOOS {
	case "darwin":
		return "  brew install python"
	case "windows":
		return "  winget install -e --id Python.Python.3.12\n" +
			"  If `winget` is unavailable, download Python from https://python.org and enable `Add Python to PATH` during setup."
	default:
		return "  sudo apt update && sudo apt install -y python3 python3-pip python3-venv    # Debian / Ubuntu\n" +
			"  sudo dnf install -y python3 python3-pip                                    # Fedora / RHEL / Rocky"
	}
}

func pythonDepsInstallHint() string {
	return pythonDepsInstallHintForGOOS(runtime.GOOS)
}

func pythonDepsInstallHintForGOOS(goos string) string {
	if goos == "windows" {
		venvDir := `"%USERPROFILE%\.csghub-lite\tools\python"`
		venvPython := `"%USERPROFILE%\.csghub-lite\tools\python\Scripts\python.exe"`
		return fmt.Sprintf(
			"  py -m venv %s\n"+
				"  %s -m pip install --upgrade pip\n"+
				"  %s -m pip install %s\n"+
				"  %s -m pip install %s\n"+
				"  csghub-lite automatically checks this virtual environment on the next run.",
			venvDir,
			venvPython,
			venvPython,
			pythonCPUOnlyTorchInstallArgs,
			venvPython,
			pythonDepsInstallArgs,
		)
	}

	venvDir := "~/.csghub-lite/tools/python"
	venvPython := venvDir + "/bin/python"
	return fmt.Sprintf(
		"  python3 -m venv %s\n"+
			"  %s -m pip install --upgrade pip\n"+
			"  %s -m pip install %s\n"+
			"  %s -m pip install %s\n"+
			"  csghub-lite automatically checks this virtual environment on the next run.",
		venvDir,
		venvPython,
		venvPython,
		pythonCPUOnlyTorchInstallArgs,
		venvPython,
		pythonDepsInstallArgs,
	)
}

func preferredPipInstallCommand() string {
	if runtime.GOOS == "windows" {
		return `"%USERPROFILE%\.csghub-lite\tools\python\Scripts\python.exe" -m pip install --upgrade`
	}
	return "~/.csghub-lite/tools/python/bin/python -m pip install --upgrade"
}

func ggufRepoInstallCommand(repoURL string) string {
	return fmt.Sprintf(
		`%s "gguf @ git+%s.git@%s#subdirectory=gguf-py"`,
		preferredPipInstallCommand(),
		repoURL,
		BundledConverterLLamacppRef,
	)
}

func ggufRepoInstallHint(region string) string {
	sources := llamaCppGGUFPySources()
	if len(sources) == 0 {
		return ""
	}
	lines := []string{
		"Install the matching `gguf-py` directly from the Gitee llama.cpp source:",
		"  " + ggufRepoInstallCommand(sources[0].repoURL),
	}
	return strings.Join(lines, "\n")
}

func sourceGGUFPySetupHint(region string) string {
	sources := llamaCppGGUFPySources()
	if len(sources) == 0 {
		return ""
	}
	lines := []string{
		fmt.Sprintf(
			"csghub-lite installs `gguf-py` from llama.cpp source tag `%s`, not from PyPI.",
			BundledConverterLLamacppRef,
		),
		fmt.Sprintf("Source: %s.", sources[0].repoURL),
	}
	return strings.Join(lines, "\n")
}

func bundledConverterVersionString() string {
	return fmt.Sprintf("llama.cpp %s (bundled revision %d)", BundledConverterLLamacppRef, bundledConverterRevision)
}

func converterContextSummary() string {
	if rawURL := strings.TrimSpace(os.Getenv("CSGHUB_LITE_CONVERTER_URL")); rawURL != "" {
		return fmt.Sprintf("Converter source: CSGHUB_LITE_CONVERTER_URL=%s", rawURL)
	}
	return fmt.Sprintf("Converter version: %s", bundledConverterVersionString())
}

func converterProgressSummary() string {
	if strings.TrimSpace(os.Getenv("CSGHUB_LITE_CONVERTER_URL")) != "" {
		return "official converter from CSGHUB_LITE_CONVERTER_URL"
	}
	return fmt.Sprintf("official converter from %s", bundledConverterVersionString())
}

func converterErrorf(format string, args ...any) error {
	return fmt.Errorf("%s\n%s", converterContextSummary(), fmt.Sprintf(format, args...))
}

func converterCacheDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".csghub-lite", "tools")
}

func managedPythonVenvDir() string {
	return filepath.Join(converterCacheDir(), "python")
}

func managedPythonVenvExecutable() string {
	if runtime.GOOS == "windows" {
		return filepath.Join(managedPythonVenvDir(), "Scripts", "python.exe")
	}
	return filepath.Join(managedPythonVenvDir(), "bin", "python")
}

func managedGGUFPyPath() string {
	return filepath.Join(bundledConverterDir(), "gguf-py")
}

func bundledConverterDir() string {
	return filepath.Join(converterCacheDir(), "bundled")
}

func remoteConverterDir() string {
	return filepath.Join(converterCacheDir(), "remote")
}

// findPythonEnv locates a suitable Python interpreter.
// Returns (pythonPath, missingDeps) where:
//   - pythonPath != "" && missingDeps == "": ready to use
//   - pythonPath != "" && missingDeps != "": Python found but packages missing
//   - pythonPath == "": no Python found at all
func findPythonEnv() (pythonPath string, missingDeps string) {
	if p := managedPythonVenvExecutable(); p != "" {
		if _, err := os.Stat(p); err == nil {
			if missing := checkPythonDeps(p); missing == "" {
				return p, ""
			}
			// Prefer reporting missing packages for the managed venv so the
			// setup hint installs into the same interpreter csghub-lite will use.
			return p, checkPythonDeps(p)
		}
	}

	if firstPython := findPythonInterpreter(); firstPython != "" {
		missing := checkPythonDeps(firstPython)
		if missing == "" {
			return firstPython, ""
		}
		return firstPython, missing
	}
	return "", ""
}

func findPythonInterpreter() string {
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

	for _, name := range candidates {
		if p, err := exec.LookPath(name); err == nil {
			if pythonVersionSupported(p) {
				return p
			}
		}
	}
	for _, p := range extraPaths {
		if _, err := os.Stat(p); err == nil {
			if pythonVersionSupported(p) {
				return p
			}
		}
	}
	return ""
}

func pythonVersionSupported(python string) bool {
	cmd := exec.Command(python, "-c", "import sys; raise SystemExit(0 if sys.version_info >= (3, 10) else 1)")
	return cmd.Run() == nil
}

// checkPythonDeps returns a comma-separated list of missing packages, or "" if all present.
func checkPythonDeps(python string) string {
	required := requiredPythonModules()
	var missing []string
	for _, pkg := range required {
		cmd := exec.Command(python, "-c", "import "+pkg)
		if cmd.Run() != nil {
			missing = append(missing, pkg)
		}
	}
	return strings.Join(missing, ", ")
}

func requiredPythonModules() []string {
	return []string{"torch", "safetensors", "transformers"}
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
	dir := bundledConverterDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("creating tools dir: %w", err)
	}
	revPath := filepath.Join(dir, "bundled_convert_hf_revision")
	dst := filepath.Join(dir, "convert_hf_to_gguf.py")
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
	dir := remoteConverterDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("creating tools dir: %w", err)
	}
	urlPath := filepath.Join(dir, "remote_convert_hf_url")
	dst := filepath.Join(dir, "convert_hf_to_gguf.py")
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
// generated GGUF file. Requires python3 with torch, safetensors, and transformers.
func ConvertPython(modelDir string, progress ProgressFunc, dtype string) (string, error) {
	if progress == nil {
		progress = func(string, int, int) {}
	}

	effectiveDType, err := resolveDType(dtype)
	if err != nil {
		return "", err
	}

	if existingPath, ok, err := FindGGUFForDType(modelDir, effectiveDType); err != nil {
		return "", err
	} else if ok {
		return existingPath, nil
	}

	basePython := findPythonInterpreter()
	if basePython == "" {
		return "", converterErrorf(
			"this checkpoint is SafeTensors-only; csghub-lite converts it to GGUF once using the official llama.cpp Python script.\n"+
				"The Python runtime and conversion packages are not bundled with the release binary.\n\n"+
				"Python 3.10+ was not found on PATH.\n"+
				"Please complete these one-time setup steps:\n"+
				"  1. Install Python 3.10+ and make sure python3 is available on PATH.\n"+
				"%s\n"+
				"  2. Install conversion deps:\n"+
				"%s\n\n"+
				"If the hub offers a GGUF build of the same model, download that instead to skip conversion.",
			pythonInstallHint(),
			pythonDepsInstallHint(),
		)
	}

	progress("Preparing Python conversion environment", 0, 0)
	python, setupOutput, setupErr := ensureManagedPythonEnv(basePython)
	if setupErr != nil {
		if setupOutput == "" {
			setupOutput = "(no setup output)"
		}
		return "", converterErrorf(
			"this checkpoint is SafeTensors-only; csghub-lite converts it to GGUF once using the official llama.cpp Python script.\n"+
				"csghub-lite tried to prepare an isolated Python environment automatically, but setup failed.\n\n"+
				"Automatic setup failed: %s\n%s\n\n"+
				"Run these one-time setup commands manually, then retry:\n"+
				"%s\n\n"+
				"If a GGUF variant exists on CSGHub or Hugging Face, use it to skip conversion.",
			setupErr,
			lastNLines(setupOutput, 12),
			pythonDepsInstallHint(),
		)
	}

	step := fmt.Sprintf("Preparing converter (%s)", bundledConverterVersionString())
	if strings.TrimSpace(os.Getenv("CSGHUB_LITE_CONVERTER_URL")) != "" {
		step = "Downloading converter from CSGHUB_LITE_CONVERTER_URL"
	}
	progress(step, 0, 0)
	script, err := ensureConverterScript()
	if err != nil {
		return "", converterErrorf("%v", err)
	}

	if sourceName, err := ensureConverterGGUFPySource(progress); err != nil {
		return "", converterErrorf(
			"this checkpoint is SafeTensors-only; csghub-lite converts it to GGUF once using the official llama.cpp Python script.\n"+
				"csghub-lite tried to prepare matching `gguf-py` from llama.cpp source, but setup failed.\n\n"+
				"Automatic gguf-py setup failed: %s\n\n"+
				"%s\n\n"+
				"If a GGUF variant exists on CSGHub or Hugging Face, use it to skip conversion.",
			err,
			sourceGGUFPySetupHint(detectLlamaCppSourceRegion()),
		)
	} else {
		progress(fmt.Sprintf("Prepared matching gguf-py from %s", sourceName), 0, 0)
	}

	outputName := generateOutputName(modelDir, effectiveDType)
	outputPath := filepath.Join(modelDir, outputName)

	progress(fmt.Sprintf("Converting with %s to GGUF (dtype: %s)", converterProgressSummary(), effectiveDType), 0, 0)
	if err := convertModelWithAutoRepair(python, script, modelDir, outputPath, effectiveDType, progress); err != nil {
		return "", err
	}

	if effectiveDType == "auto" {
		if existingPath, ok, err := FindGGUFForDType(modelDir, "auto"); err != nil {
			return "", err
		} else if ok {
			outputPath = existingPath
		} else {
			return "", converterErrorf("converter finished but output file not found for dtype %q", effectiveDType)
		}
	} else if _, err := os.Stat(outputPath); err != nil {
		return "", converterErrorf("converter finished but output file not found: %s", outputPath)
	}

	if hasVisionConfig(modelDir) {
		if _, ok, err := FindMMProjForDType(modelDir, effectiveDType); err != nil {
			return "", err
		} else if !ok {
			progress(fmt.Sprintf("Converting vision encoder (mmproj) to GGUF (dtype: %s)", effectiveDType), 0, 0)
			mmOut, mmErr := runMMProjConverter(python, script, modelDir, effectiveDType)
			if mmErr != nil {
				log.Printf("mmproj conversion failed (non-fatal): %s\n%s", mmErr, lastNLines(mmOut, 5))
			} else {
				log.Printf("mmproj conversion succeeded")
			}
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

func ensureManagedPythonEnv(basePython string) (string, string, error) {
	python := managedPythonVenvExecutable()
	if python == "" {
		return "", "", fmt.Errorf("managed Python executable path is empty")
	}
	if _, err := os.Stat(python); err != nil {
		output, runErr := runCommand(basePython, "-m", "venv", managedPythonVenvDir())
		if runErr != nil {
			return "", output, fmt.Errorf("creating Python virtual environment: %w", runErr)
		}
	} else if missing := checkPythonDeps(python); missing == "" {
		return python, "", nil
	}

	var combined []string
	steps := [][]string{
		{"-m", "pip", "install", "--upgrade", "pip"},
		{"-m", "pip", "install", "--upgrade", "--index-url", "https://download.pytorch.org/whl/cpu", "torch"},
		{"-m", "pip", "install", "--upgrade", "safetensors", "transformers"},
	}
	for _, args := range steps {
		output, err := runPythonPipCommand(python, args...)
		if output != "" {
			combined = append(combined, output)
		}
		if err != nil {
			return "", strings.Join(combined, "\n"), err
		}
	}
	if missing := checkPythonDeps(python); missing != "" {
		return "", strings.Join(combined, "\n"), fmt.Errorf("missing Python packages after automatic install: %s", missing)
	}
	return python, strings.Join(combined, "\n"), nil
}

func runPythonPipCommand(python string, args ...string) (string, error) {
	output, err := runCommand(python, args...)
	if err != nil {
		return output, fmt.Errorf("%s %s: %w", python, strings.Join(args, " "), err)
	}
	return output, nil
}

func runCommand(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Env = append(os.Environ(), "PIP_DISABLE_PIP_VERSION_CHECK=1")
	output, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(output)), err
}

func ensureConverterGGUFPySource(progress ProgressFunc) (string, error) {
	progress("Preparing matching gguf-py from Gitee llama.cpp source", 0, 0)
	return ensureBundledGGUFPy()
}

func convertModelWithAutoRepair(python, script, modelDir, outputPath, dtype string, progress ProgressFunc) error {
	output, err := runModelConverter(python, script, modelDir, outputPath, dtype)
	if err == nil {
		return nil
	}

	repair := attemptConverterAutoRepair(python, output, progress)
	if repair.attempted && repair.succeeded {
		_ = os.Remove(outputPath)
		progress(fmt.Sprintf("Retrying converter after automatic repair (dtype: %s)", dtype), 0, 0)
		output, err = runModelConverter(python, script, modelDir, outputPath, dtype)
		if err == nil {
			return nil
		}
	}

	_ = os.Remove(outputPath)
	return formatConverterFailure(err, output, repair.note)
}

func runModelConverter(python, script, modelDir, outputPath, dtype string) (string, error) {
	cmd := exec.Command(python, script, modelDir,
		"--outfile", outputPath,
		"--outtype", dtype,
	)
	cmd.Env = converterPythonEnv()
	output, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(output)), err
}

func runMMProjConverter(python, script, modelDir, dtype string) (string, error) {
	cmd := exec.Command(python, script, modelDir,
		"--outtype", dtype,
		"--mmproj",
	)
	cmd.Dir = modelDir
	cmd.Env = converterPythonEnv()
	output, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(output)), err
}

func converterPythonEnv() []string {
	env := os.Environ()
	ggufPath := managedGGUFPyPath()
	if existing := os.Getenv("PYTHONPATH"); existing != "" {
		env = append(env, "PYTHONPATH="+ggufPath+string(os.PathListSeparator)+existing)
	} else {
		env = append(env, "PYTHONPATH="+ggufPath)
	}
	return env
}

func attemptConverterAutoRepair(python, combined string, progress ProgressFunc) converterRepairResult {
	plan := repairPlanForConverterFailure(combined)
	if !plan.installBundledGGUFPy && len(plan.upgradePackages) == 0 {
		return converterRepairResult{}
	}

	var notes []string
	var failures []string
	var otherPackages []string
	for _, pkg := range plan.upgradePackages {
		if pkg == "gguf" {
			continue
		}
		otherPackages = append(otherPackages, pkg)
	}

	if plan.installBundledGGUFPy {
		progress("Preparing matching gguf-py from Gitee llama.cpp source", 0, 0)
		sourceName, err := ensureBundledGGUFPy()
		if err != nil {
			failures = append(failures, fmt.Sprintf(
				"csghub-lite detected that this bundled converter needs matching `gguf-py` from llama.cpp tag `%s`.\n"+
					"It tried Gitee source: %s.\n\n"+
					"Automatic gguf-py download failed: %s\n\n"+
					"%s",
				BundledConverterLLamacppRef,
				llamaCppGiteeRepo,
				err,
				ggufRepoInstallHint(regionCN),
			))
		} else {
			notes = append(notes, fmt.Sprintf("prepared matching gguf-py from %s", sourceName))
		}
	}

	if len(otherPackages) > 0 {
		progress(fmt.Sprintf("Upgrading Python package%s: %s", pluralSuffix(len(otherPackages)), strings.Join(otherPackages, ", ")), 0, 0)
		pipOutput, pipErr := upgradePythonPackages(python, otherPackages)
		command := fmt.Sprintf("%s -m pip install --upgrade %s", python, strings.Join(otherPackages, " "))

		if pipErr != nil {
			pipSummary := lastNLines(pipOutput, 10)
			if pipSummary == "" {
				pipSummary = "(no pip output)"
			}
			failures = append(failures, fmt.Sprintf(
				"csghub-lite tried to run:\n  %s\n\n"+
					"Automatic package upgrade failed: %s\n%s",
				command,
				pipErr,
				pipSummary,
			))
		} else {
			notes = append(notes, fmt.Sprintf("upgraded %s", strings.Join(otherPackages, ", ")))
		}
	}

	if len(notes) == 0 {
		return converterRepairResult{
			attempted: true,
			note:      repairFailureNote(failures),
		}
	}

	return converterRepairResult{
		attempted: true,
		succeeded: true,
		note:      repairSuccessNote(notes, failures),
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

func repairPlanForConverterFailure(combined string) converterRepairPlan {
	if combined == "" {
		return converterRepairPlan{}
	}

	var plan converterRepairPlan
	add := func(pkg string) {
		for _, existing := range plan.upgradePackages {
			if existing == pkg {
				return
			}
		}
		plan.upgradePackages = append(plan.upgradePackages, pkg)
	}

	lower := strings.ToLower(combined)
	if (strings.Contains(combined, "AttributeError") &&
		(strings.Contains(combined, "MODEL_ARCH") || strings.Contains(combined, "gguf."))) ||
		strings.Contains(lower, "no module named 'gguf'") ||
		strings.Contains(lower, "no module named \"gguf\"") {
		plan.installBundledGGUFPy = true
	}
	if strings.Contains(combined, "Transformers does not recognize this architecture") ||
		strings.Contains(combined, "pip install --upgrade transformers") ||
		strings.Contains(combined, "pip install git+https://github.com/huggingface/transformers.git") ||
		strings.Contains(lower, "no module named 'transformers.models.") {
		add("transformers")
	}

	return plan
}

func repairFailureNote(failures []string) string {
	if len(failures) == 0 {
		return ""
	}
	return "\n\nAutomatic repair failed:\n\n" + strings.Join(failures, "\n\n")
}

func repairSuccessNote(notes, failures []string) string {
	note := fmt.Sprintf(
		"\n\ncsghub-lite auto-repaired the converter environment (%s) and retried once.",
		strings.Join(notes, ", "),
	)
	if len(failures) > 0 {
		note += "\n\nSome automatic repair steps still failed, so manual cleanup may still be needed:\n\n" +
			strings.Join(failures, "\n\n")
	}
	return note
}

func detectLlamaCppSourceRegion() string {
	if region := strings.ToUpper(strings.TrimSpace(os.Getenv("CSGHUB_LITE_REGION"))); region != "" {
		if region == regionCN {
			return regionCN
		}
		return regionINTL
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get("https://ipinfo.io/country")
	if err == nil {
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			body, readErr := io.ReadAll(io.LimitReader(resp.Body, 16))
			if readErr == nil {
				country := strings.TrimSpace(string(body))
				if country == regionCN {
					return regionCN
				}
				if country != "" {
					return regionINTL
				}
			}
		}
	}

	return regionCN
}

func llamaCppSources(region string) []llamaCppSource {
	gitee := llamaCppSource{
		name:       "Gitee mirror",
		repoURL:    llamaCppGiteeRepo,
		archiveURL: fmt.Sprintf("%s/archive/refs/tags/%s.tar.gz", llamaCppGiteeRepo, BundledConverterLLamacppRef),
	}
	github := llamaCppSource{
		name:       "GitHub upstream",
		repoURL:    llamaCppGitHubRepo,
		archiveURL: fmt.Sprintf("%s/archive/refs/tags/%s.tar.gz", llamaCppGitHubRepo, BundledConverterLLamacppRef),
	}
	if strings.EqualFold(region, regionCN) {
		return []llamaCppSource{gitee, github}
	}
	return []llamaCppSource{github, gitee}
}

func llamaCppGGUFPySources() []llamaCppSource {
	return []llamaCppSource{{
		name:       "Gitee mirror",
		repoURL:    llamaCppGiteeRepo,
		archiveURL: fmt.Sprintf("%s/archive/refs/tags/%s.tar.gz", llamaCppGiteeRepo, BundledConverterLLamacppRef),
	}}
}

func llamaCppSourceNames(region string) []string {
	sources := llamaCppSources(region)
	names := make([]string, 0, len(sources))
	for _, source := range sources {
		names = append(names, source.name)
	}
	return names
}

func ensureBundledGGUFPy() (string, error) {
	dir := bundledConverterDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("creating bundled converter dir: %w", err)
	}

	stampPath := filepath.Join(dir, "bundled_gguf_py_ref")
	sourcePath := filepath.Join(dir, "bundled_gguf_py_source")
	dst := filepath.Join(dir, "gguf-py")
	wantStamp := bundledConverterStamp()

	if prev, err := os.ReadFile(stampPath); err == nil && string(prev) == wantStamp {
		if _, err := os.Stat(filepath.Join(dst, "gguf", "__init__.py")); err == nil {
			if source, err := os.ReadFile(sourcePath); err == nil && strings.TrimSpace(string(source)) != "" {
				return strings.TrimSpace(string(source)), nil
			}
			return "cached llama.cpp source", nil
		}
	}

	archiveFile, err := os.CreateTemp(dir, "llama.cpp-*.tar.gz")
	if err != nil {
		return "", fmt.Errorf("creating llama.cpp archive temp file: %w", err)
	}
	archivePath := archiveFile.Name()
	archiveFile.Close()
	defer os.Remove(archivePath)

	sourceName, err := downloadLlamaCppArchive(archivePath, llamaCppGGUFPySources())
	if err != nil {
		return "", err
	}

	tmpDir, err := os.MkdirTemp(dir, "gguf-py-*")
	if err != nil {
		return "", fmt.Errorf("creating gguf-py temp dir: %w", err)
	}
	defer func() {
		if tmpDir != "" {
			os.RemoveAll(tmpDir)
		}
	}()

	if err := extractGGUFPyFromTarGz(archivePath, tmpDir); err != nil {
		return "", err
	}

	if err := os.RemoveAll(dst); err != nil {
		return "", fmt.Errorf("removing old gguf-py: %w", err)
	}
	if err := os.Rename(tmpDir, dst); err != nil {
		return "", fmt.Errorf("installing gguf-py: %w", err)
	}
	tmpDir = ""

	if err := os.WriteFile(stampPath, []byte(wantStamp), 0o644); err != nil {
		return "", fmt.Errorf("writing gguf-py stamp: %w", err)
	}
	if err := os.WriteFile(sourcePath, []byte(sourceName), 0o644); err != nil {
		return "", fmt.Errorf("writing gguf-py source: %w", err)
	}

	return sourceName, nil
}

func downloadLlamaCppArchive(dst string, sources []llamaCppSource) (string, error) {
	client := &http.Client{Timeout: 5 * time.Minute}
	var failures []string
	for _, source := range sources {
		if err := downloadURLToFile(client, source.archiveURL, dst); err == nil {
			return source.name, nil
		} else {
			failures = append(failures, fmt.Sprintf("%s: %v", source.name, err))
		}
	}
	return "", fmt.Errorf("downloading llama.cpp archive failed: %s", strings.Join(failures, "; "))
}

func downloadURLToFile(client *http.Client, rawURL, dst string) error {
	resp, err := client.Get(rawURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	f, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err := io.Copy(f, resp.Body); err != nil {
		return err
	}
	return nil
}

func extractGGUFPyFromTarGz(archivePath, dst string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("opening llama.cpp archive: %w", err)
	}
	defer f.Close()

	gzr, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("opening llama.cpp gzip stream: %w", err)
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	found := false
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("reading llama.cpp archive: %w", err)
		}

		name := strings.TrimPrefix(hdr.Name, "./")
		idx := strings.Index(name, "/gguf-py/")
		if idx < 0 {
			continue
		}

		rel := name[idx+len("/gguf-py/"):]
		if rel == "" {
			found = true
			continue
		}

		target := filepath.Join(dst, filepath.FromSlash(rel))
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, os.FileMode(hdr.Mode)); err != nil {
				return fmt.Errorf("creating gguf-py dir: %w", err)
			}
			found = true
		case tar.TypeReg, tar.TypeRegA:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return fmt.Errorf("creating gguf-py file dir: %w", err)
			}
			mode := os.FileMode(hdr.Mode)
			if mode == 0 {
				mode = 0o644
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
			if err != nil {
				return fmt.Errorf("creating gguf-py file: %w", err)
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return fmt.Errorf("writing gguf-py file: %w", err)
			}
			if err := out.Close(); err != nil {
				return fmt.Errorf("closing gguf-py file: %w", err)
			}
			found = true
		}
	}

	if !found {
		return fmt.Errorf("llama.cpp archive did not contain gguf-py")
	}
	if _, err := os.Stat(filepath.Join(dst, "gguf", "__init__.py")); err != nil {
		return fmt.Errorf("extracted gguf-py is incomplete: %w", err)
	}

	return nil
}

func lastNLines(s string, n int) string {
	lines := strings.Split(s, "\n")
	if len(lines) <= n {
		return s
	}
	return strings.Join(lines[len(lines)-n:], "\n")
}

func formatConverterFailure(err error, output string, repairNote string) error {
	return converterErrorf(
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
				"csghub-lite uses matching `gguf-py` from Gitee llama.cpp tag `%s`, not PyPI.\n"+
				"%s\n"+
				"To reset the bundled copy, delete the bundled converter cache under %s\n",
			BundledConverterLLamacppRef,
			ggufRepoInstallHint(regionCN),
			bundledConverterDir(),
		)
	}
	if strings.Contains(combined, "Transformers does not recognize this architecture") ||
		strings.Contains(combined, "pip install --upgrade transformers") ||
		strings.Contains(combined, "pip install git+https://github.com/huggingface/transformers.git") {
		return "\n\nThe installed `transformers` package looks too old for this model.\n" +
			"If the automatic upgrade did not fix it, run:\n" +
			"  " + preferredPipInstallCommand() + " transformers\n"
	}
	return ""
}
