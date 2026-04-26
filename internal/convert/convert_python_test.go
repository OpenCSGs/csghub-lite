package convert

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestRepairPlanForConverterFailureGGUF(t *testing.T) {
	output := `
INFO:hf-to-gguf:Model architecture: Gemma4ForConditionalGeneration
model_arch = gguf.MODEL_ARCH.GEMMA4
AttributeError: GEMMA4. Did you mean: 'GEMMA'?
`

	got := repairPlanForConverterFailure(output)
	want := converterRepairPlan{
		installBundledGGUFPy: true,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("repairPlanForConverterFailure() = %#v, want %#v", got, want)
	}
}

func TestRepairPlanForConverterFailureMissingGGUFModule(t *testing.T) {
	output := `
Traceback (most recent call last):
  File "convert_hf_to_gguf.py", line 30, in <module>
    import gguf
ModuleNotFoundError: No module named 'gguf'
`

	got := repairPlanForConverterFailure(output)
	want := converterRepairPlan{
		installBundledGGUFPy: true,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("repairPlanForConverterFailure() = %#v, want %#v", got, want)
	}
}

func TestRepairPlanForConverterFailureTransformers(t *testing.T) {
	output := `
The checkpoint you are trying to load has model type "gemma4" but Transformers does not recognize this architecture.
You can update Transformers with the command "pip install --upgrade transformers".
`

	got := repairPlanForConverterFailure(output)
	want := converterRepairPlan{upgradePackages: []string{"transformers"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("repairPlanForConverterFailure() = %#v, want %#v", got, want)
	}
}

func TestRepairPlanForConverterFailureSentencePiece(t *testing.T) {
	output := `
Traceback (most recent call last):
  File "convert_hf_to_gguf.py", line 1652, in _create_vocab_sentencepiece
    from sentencepiece import SentencePieceProcessor
ModuleNotFoundError: No module named 'sentencepiece'
`

	got := repairPlanForConverterFailure(output)
	want := converterRepairPlan{upgradePackages: []string{"sentencepiece"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("repairPlanForConverterFailure() = %#v, want %#v", got, want)
	}
}

func TestRepairPlanForConverterFailureDeduplicates(t *testing.T) {
	output := `
The checkpoint you are trying to load has model type "gemma4" but Transformers does not recognize this architecture.
You can update Transformers with the command "pip install --upgrade transformers".
model_arch = gguf.MODEL_ARCH.GEMMA4
AttributeError: GEMMA4. Did you mean: 'GEMMA'?
`

	got := repairPlanForConverterFailure(output)
	want := converterRepairPlan{
		installBundledGGUFPy: true,
		upgradePackages:      []string{"transformers"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("repairPlanForConverterFailure() = %#v, want %#v", got, want)
	}
}

func TestLlamaCppSourcesPreferGiteeInCN(t *testing.T) {
	got := llamaCppSources(regionCN)
	if len(got) != 2 {
		t.Fatalf("len(llamaCppSources(CN)) = %d, want 2", len(got))
	}
	if got[0].name != "Gitee mirror" || got[1].name != "GitHub upstream" {
		t.Fatalf("llamaCppSources(CN) order = %#v", got)
	}
}

func TestLlamaCppSourcesPreferGitHubOutsideCN(t *testing.T) {
	got := llamaCppSources(regionINTL)
	if len(got) != 2 {
		t.Fatalf("len(llamaCppSources(INTL)) = %d, want 2", len(got))
	}
	if got[0].name != "GitHub upstream" || got[1].name != "Gitee mirror" {
		t.Fatalf("llamaCppSources(INTL) order = %#v", got)
	}
}

func TestGGUFRepoInstallHintIncludesCopyableCommands(t *testing.T) {
	got := ggufRepoInstallHint(regionCN)
	if !strings.Contains(got, `git+https://gitee.com/xzgan/llama.cpp.git@`+BundledConverterLLamacppRef+`#subdirectory=gguf-py`) {
		t.Fatalf("ggufRepoInstallHint(CN) missing Gitee command: %q", got)
	}
	if strings.Contains(got, "github.com/ggml-org") || strings.Contains(got, "pip install gguf") {
		t.Fatalf("ggufRepoInstallHint(CN) should only use Gitee source, got: %q", got)
	}
}

func TestPythonDepsInstallHintUsesManagedVenv(t *testing.T) {
	got := pythonDepsInstallHintForGOOS("darwin")
	if strings.Contains(got, "pip3 install") {
		t.Fatalf("pythonDepsInstallHintForGOOS(darwin) should not suggest global pip3 install: %q", got)
	}
	for _, want := range []string{
		"python3 -m venv ~/.csghub-lite/tools/python",
		"~/.csghub-lite/tools/python/bin/python -m pip install --index-url https://download.pytorch.org/whl/cpu torch",
		"~/.csghub-lite/tools/python/bin/python -m pip install safetensors transformers sentencepiece",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("pythonDepsInstallHintForGOOS(darwin) missing %q in %q", want, got)
		}
	}
}

func TestPythonNotFoundOrUnsupportedMessageMentionsOldVersion(t *testing.T) {
	got := pythonNotFoundOrUnsupportedMessage("/usr/bin/python3 (Python 3.8.18)")
	if !strings.Contains(got, "Python 3.8.18") || !strings.Contains(got, "requires Python 3.9+") {
		t.Fatalf("unsupported Python message = %q", got)
	}
}

func TestPythonDepsInstallHintUsesManagedVenvOnWindows(t *testing.T) {
	got := pythonDepsInstallHintForGOOS("windows")
	for _, want := range []string{
		`py -m venv "%USERPROFILE%\.csghub-lite\tools\python"`,
		`"%USERPROFILE%\.csghub-lite\tools\python\Scripts\python.exe" -m pip install --index-url https://download.pytorch.org/whl/cpu torch`,
		`"%USERPROFILE%\.csghub-lite\tools\python\Scripts\python.exe" -m pip install safetensors transformers sentencepiece`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("pythonDepsInstallHintForGOOS(windows) missing %q in %q", want, got)
		}
	}
}

func TestHintForConverterScriptFailureIncludesGGUFRepoInstallExample(t *testing.T) {
	output := `
INFO:hf-to-gguf:Model architecture: Gemma4ForConditionalGeneration
model_arch = gguf.MODEL_ARCH.GEMMA4
AttributeError: GEMMA4. Did you mean: 'GEMMA'?
`
	got := hintForConverterScriptFailure(output)
	if !strings.Contains(got, `git+https://gitee.com/xzgan/llama.cpp.git@`+BundledConverterLLamacppRef+`#subdirectory=gguf-py`) {
		t.Fatalf("hintForConverterScriptFailure() missing Gitee install example: %q", got)
	}
}

func TestConverterErrorfIncludesBundledVersion(t *testing.T) {
	got := converterErrorf("example failure").Error()
	if !strings.Contains(got, "Converter version: llama.cpp "+BundledConverterLLamacppRef) {
		t.Fatalf("converterErrorf() missing llama.cpp tag: %q", got)
	}
	if !strings.Contains(got, fmt.Sprintf("bundled revision %d", bundledConverterRevision)) {
		t.Fatalf("converterErrorf() missing bundled revision: %q", got)
	}
}

func TestConverterErrorfUsesCustomConverterSource(t *testing.T) {
	t.Setenv("CSGHUB_LITE_CONVERTER_URL", "https://example.com/convert_hf_to_gguf.py")
	got := converterErrorf("example failure").Error()
	if !strings.Contains(got, "Converter source: CSGHUB_LITE_CONVERTER_URL=https://example.com/convert_hf_to_gguf.py") {
		t.Fatalf("converterErrorf() missing custom converter source: %q", got)
	}
}

func TestFormatConverterFailureIncludesConverterVersion(t *testing.T) {
	got := formatConverterFailure(fmt.Errorf("exit status 1"), "Traceback\nline2", "").Error()
	if !strings.Contains(got, "Converter version: llama.cpp "+BundledConverterLLamacppRef) {
		t.Fatalf("formatConverterFailure() missing llama.cpp tag: %q", got)
	}
}

func TestDetectLlamaCppSourceRegionUsesEnvOverride(t *testing.T) {
	t.Setenv("CSGHUB_LITE_REGION", "CN")
	if got := detectLlamaCppSourceRegion(); got != regionCN {
		t.Fatalf("detectLlamaCppSourceRegion() with CN = %q, want %q", got, regionCN)
	}

	t.Setenv("CSGHUB_LITE_REGION", "intl")
	if got := detectLlamaCppSourceRegion(); got != regionINTL {
		t.Fatalf("detectLlamaCppSourceRegion() with intl = %q, want %q", got, regionINTL)
	}
}
