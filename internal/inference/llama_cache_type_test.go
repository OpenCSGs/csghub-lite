package inference

import (
	"slices"
	"testing"
)

func TestNormalizeCacheTypeAcceptsAllowedValues(t *testing.T) {
	got, err := NormalizeCacheType("Q8_0")
	if err != nil {
		t.Fatalf("NormalizeCacheType returned error: %v", err)
	}
	if got != "q8_0" {
		t.Fatalf("NormalizeCacheType = %q, want q8_0", got)
	}
}

func TestNormalizeCacheTypeRejectsUnknownValues(t *testing.T) {
	if _, err := NormalizeCacheType("fp8"); err == nil {
		t.Fatal("expected invalid cache type error")
	}
}

func TestAllowedCacheTypesIncludesKVDefaults(t *testing.T) {
	allowed := AllowedCacheTypes()
	for _, want := range []string{"f16", "bf16", "q8_0"} {
		if !slices.Contains(allowed, want) {
			t.Fatalf("AllowedCacheTypes missing %q: %#v", want, allowed)
		}
	}
}
