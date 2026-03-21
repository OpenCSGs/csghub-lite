package convert

import "testing"

func TestBundledConverterPyPresent(t *testing.T) {
	if len(bundledConverterPy) < 10_000 {
		t.Fatalf("embedded convert_hf_to_gguf.py looks missing or truncated (got %d bytes)", len(bundledConverterPy))
	}
}
