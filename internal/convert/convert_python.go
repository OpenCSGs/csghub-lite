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
	pythonDepsInstallArgs = "torch safetensors gguf transformers"
	regionCN              = "CN"
	regionINTL            = "INTL"
	llamaCppGitHubRepo    = "https://github.com/ggml-org/llama.cpp"
	llamaCppGiteeRepo     = "https://gitee.com/xzgan/llama.cpp"
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
	required := []string{"torch", "safetensors", "transformers"}
	if strings.TrimSpace(os.Getenv("CSGHUB_LITE_CONVERTER_URL")) != "" {
		required = append(required, "gguf")
	}
	return required
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

	outputName := generateOutputName(modelDir, effectiveDType)
	outputPath := filepath.Join(modelDir, outputName)

	progress("Converting with official converter", 0, 0)
	if err := convertModelWithAutoRepair(python, script, modelDir, outputPath, effectiveDType, progress); err != nil {
		return "", err
	}

	if effectiveDType == "auto" {
		if existingPath, ok, err := FindGGUFForDType(modelDir, "auto"); err != nil {
			return "", err
		} else if ok {
			outputPath = existingPath
		} else {
			return "", fmt.Errorf("converter finished but output file not found for dtype %q", effectiveDType)
		}
	} else if _, err := os.Stat(outputPath); err != nil {
		return "", fmt.Errorf("converter finished but output file not found: %s", outputPath)
	}

	if hasVisionConfig(modelDir) {
		if _, ok, err := FindMMProjForDType(modelDir, effectiveDType); err != nil {
			return "", err
		} else if !ok {
			progress("Converting vision encoder (mmproj)", 0, 0)
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

func convertModelWithAutoRepair(python, script, modelDir, outputPath, dtype string, progress ProgressFunc) error {
	output, err := runModelConverter(python, script, modelDir, outputPath, dtype)
	if err == nil {
		return nil
	}

	repair := attemptConverterAutoRepair(python, output, progress)
	if repair.attempted && repair.succeeded {
		_ = os.Remove(outputPath)
		progress("Retrying converter after automatic repair", 0, 0)
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
	output, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(output)), err
}

func runMMProjConverter(python, script, modelDir, dtype string) (string, error) {
	cmd := exec.Command(python, script, modelDir,
		"--outtype", dtype,
		"--mmproj",
	)
	cmd.Dir = modelDir
	output, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(output)), err
}

func attemptConverterAutoRepair(python, combined string, progress ProgressFunc) converterRepairResult {
	plan := repairPlanForConverterFailure(combined)
	if !plan.installBundledGGUFPy && len(plan.upgradePackages) == 0 {
		return converterRepairResult{}
	}

	var notes []string
	if plan.installBundledGGUFPy {
		region := detectLlamaCppSourceRegion()
		progress("Preparing matching gguf-py from llama.cpp", 0, 0)
		sourceName, err := ensureBundledGGUFPy(region)
		if err != nil {
			return converterRepairResult{
				attempted: true,
				note: fmt.Sprintf(
					"\n\ncsghub-lite detected that this bundled converter needs matching `gguf-py` from llama.cpp tag `%s`.\n"+
						"It tried these sources in order for region `%s`: %s.\n\n"+
						"Automatic repair failed: %s",
					BundledConverterLLamacppRef,
					region,
					strings.Join(llamaCppSourceNames(region), ", "),
					err,
				),
			}
		}
		notes = append(notes, fmt.Sprintf("prepared matching gguf-py from %s", sourceName))
	}

	if len(plan.upgradePackages) > 0 {
		progress(fmt.Sprintf("Upgrading Python package%s: %s", pluralSuffix(len(plan.upgradePackages)), strings.Join(plan.upgradePackages, ", ")), 0, 0)
		pipOutput, pipErr := upgradePythonPackages(python, plan.upgradePackages)
		command := fmt.Sprintf("%s -m pip install --upgrade %s", python, strings.Join(plan.upgradePackages, " "))

		if pipErr != nil {
			pipSummary := lastNLines(pipOutput, 10)
			if pipSummary == "" {
				pipSummary = "(no pip output)"
			}
			note := fmt.Sprintf(
				"\n\ncsghub-lite detected a Python package version mismatch and tried to run:\n  %s\n\n"+
					"Automatic upgrade failed: %s\n%s",
				command,
				pipErr,
				pipSummary,
			)
			if len(notes) > 0 {
				note = fmt.Sprintf("\n\ncsghub-lite auto-repaired part of the converter environment (%s), but a follow-up package upgrade failed.%s",
					strings.Join(notes, ", "),
					note,
				)
			}
			return converterRepairResult{
				attempted: true,
				note:      note,
			}
		}

		notes = append(notes, fmt.Sprintf("upgraded %s", strings.Join(plan.upgradePackages, ", ")))
	}

	return converterRepairResult{
		attempted: true,
		succeeded: true,
		note: fmt.Sprintf(
			"\n\ncsghub-lite auto-repaired the converter environment (%s) and retried once.",
			strings.Join(notes, ", "),
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
		archiveURL: fmt.Sprintf("%s/archive/refs/tags/%s.tar.gz", llamaCppGiteeRepo, BundledConverterLLamacppRef),
	}
	github := llamaCppSource{
		name:       "GitHub upstream",
		archiveURL: fmt.Sprintf("%s/archive/refs/tags/%s.tar.gz", llamaCppGitHubRepo, BundledConverterLLamacppRef),
	}
	if strings.EqualFold(region, regionCN) {
		return []llamaCppSource{gitee, github}
	}
	return []llamaCppSource{github, gitee}
}

func llamaCppSourceNames(region string) []string {
	sources := llamaCppSources(region)
	names := make([]string, 0, len(sources))
	for _, source := range sources {
		names = append(names, source.name)
	}
	return names
}

func ensureBundledGGUFPy(region string) (string, error) {
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

	sourceName, err := downloadLlamaCppArchive(archivePath, region)
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

func downloadLlamaCppArchive(dst, region string) (string, error) {
	client := &http.Client{Timeout: 5 * time.Minute}
	var failures []string
	for _, source := range llamaCppSources(region) {
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
				"csghub-lite now prefers matching `gguf-py` from llama.cpp tag `%s` (CN: %s, INTL: %s).\n"+
				"If the automatic repair did not fix it, point CSGHUB_LITE_REGION to `CN` or `INTL` and retry.\n"+
				"To reset the bundled copy, delete the bundled converter cache under %s\n",
			BundledConverterLLamacppRef,
			llamaCppGiteeRepo,
			llamaCppGitHubRepo,
			bundledConverterDir(),
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
