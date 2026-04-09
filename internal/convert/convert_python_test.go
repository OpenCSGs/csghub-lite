package convert

import (
	"reflect"
	"testing"
)

func TestPackagesToAutoUpgradeForConverterFailureGGUF(t *testing.T) {
	output := `
INFO:hf-to-gguf:Model architecture: Gemma4ForConditionalGeneration
model_arch = gguf.MODEL_ARCH.GEMMA4
AttributeError: GEMMA4. Did you mean: 'GEMMA'?
`

	got := packagesToAutoUpgradeForConverterFailure(output)
	want := []string{"gguf"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("packagesToAutoUpgradeForConverterFailure() = %v, want %v", got, want)
	}
}

func TestPackagesToAutoUpgradeForConverterFailureTransformers(t *testing.T) {
	output := `
The checkpoint you are trying to load has model type "gemma4" but Transformers does not recognize this architecture.
You can update Transformers with the command "pip install --upgrade transformers".
`

	got := packagesToAutoUpgradeForConverterFailure(output)
	want := []string{"transformers"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("packagesToAutoUpgradeForConverterFailure() = %v, want %v", got, want)
	}
}

func TestPackagesToAutoUpgradeForConverterFailureDeduplicates(t *testing.T) {
	output := `
The checkpoint you are trying to load has model type "gemma4" but Transformers does not recognize this architecture.
You can update Transformers with the command "pip install --upgrade transformers".
model_arch = gguf.MODEL_ARCH.GEMMA4
AttributeError: GEMMA4. Did you mean: 'GEMMA'?
`

	got := packagesToAutoUpgradeForConverterFailure(output)
	want := []string{"gguf", "transformers"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("packagesToAutoUpgradeForConverterFailure() = %v, want %v", got, want)
	}
}
