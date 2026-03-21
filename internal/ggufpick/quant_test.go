package ggufpick

import (
	"reflect"
	"testing"
)

func TestQuantRank(t *testing.T) {
	tests := []struct {
		name string
		want int
	}{
		{"Qwen3-0.6B-Q8_0.gguf", quantRanks["q8_0"]},
		{"model-Q4_K_M.gguf", quantRanks["q4_k_m"]},
		{"Llama-3-8B-Q4_K_M-00001-of-00003.gguf", quantRanks["q4_k_m"]},
		{"weights-f16.gguf", quantRanks["f16"]},
		{"x-bf16.gguf", quantRanks["bf16"]},
		{"x-f32.gguf", quantRanks["f32"]},
		{"unknown.gguf", -1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if g := QuantRank(tt.name); g != tt.want {
				t.Errorf("QuantRank(%q) = %d, want %d", tt.name, g, tt.want)
			}
		})
	}
}

func TestFilterWeightGGUFFiles(t *testing.T) {
	entries := []FileEntry{
		{Path: "a/Q4_0.gguf", Name: "Q4_0.gguf", Size: 100},
		{Path: "a/Q8_0.gguf", Name: "Q8_0.gguf", Size: 200},
		{Path: "a/Q4_K_M.gguf", Name: "Q4_K_M.gguf", Size: 150},
	}
	got := FilterWeightGGUFFiles(entries)
	if len(got) != 1 || got[0].Name != "Q8_0.gguf" {
		t.Errorf("FilterWeightGGUFFiles = %#v, want single Q8_0", got)
	}

	sharded := []FileEntry{
		{Path: "M-Q4_0-00001-of-00002.gguf", Name: "M-Q4_0-00001-of-00002.gguf"},
		{Path: "M-Q4_0-00002-of-00002.gguf", Name: "M-Q4_0-00002-of-00002.gguf"},
		{Path: "M-Q8_0-00001-of-00002.gguf", Name: "M-Q8_0-00001-of-00002.gguf"},
		{Path: "M-Q8_0-00002-of-00002.gguf", Name: "M-Q8_0-00002-of-00002.gguf"},
	}
	got2 := FilterWeightGGUFFiles(sharded)
	wantPaths := map[string]bool{
		"M-Q8_0-00001-of-00002.gguf": true,
		"M-Q8_0-00002-of-00002.gguf": true,
	}
	if len(got2) != 2 {
		t.Fatalf("len = %d, want 2: %#v", len(got2), got2)
	}
	for _, e := range got2 {
		if !wantPaths[e.Path] {
			t.Errorf("unexpected path %q", e.Path)
		}
	}
}

func TestFilterWeightGGUFFiles_unknownOnlyNoOp(t *testing.T) {
	entries := []FileEntry{
		{Path: "a.gguf", Name: "a.gguf"},
		{Path: "b.gguf", Name: "b.gguf"},
	}
	got := FilterWeightGGUFFiles(entries)
	if !reflect.DeepEqual(got, entries) {
		t.Errorf("expected unchanged, got %#v", got)
	}
}

func TestBestWeightGGUFName(t *testing.T) {
	names := []string{"x-Q4_0.gguf", "x-Q8_0.gguf", "x-Q4_K_M.gguf"}
	if g := BestWeightGGUFName(names); g != "x-Q8_0.gguf" {
		t.Errorf("got %q, want x-Q8_0.gguf", g)
	}
}
