package convert

import (
	"strings"
	"testing"
)

func TestBundledConverterPyPresent(t *testing.T) {
	if len(bundledConverterPy) < 10_000 {
		t.Fatalf("embedded convert_hf_to_gguf.py looks missing or truncated (got %d bytes)", len(bundledConverterPy))
	}
}

func TestBundledConverterIncludesMiniMaxPatchedTokenizerHash(t *testing.T) {
	const patchedHash = "a77756c3cc91392f442c5b99e414be8020d53ae31460de90754b4fcf5cc84a2d"
	if !strings.Contains(string(bundledConverterPy), patchedHash) {
		t.Fatalf("bundled converter is missing patched MiniMax tokenizer hash %q", patchedHash)
	}
}
