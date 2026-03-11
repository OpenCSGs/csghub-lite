package model

import "testing"

func TestLocalModel_FullName(t *testing.T) {
	m := &LocalModel{
		Namespace: "OpenCSG",
		Name:      "csg-wukong-1B",
	}
	if got := m.FullName(); got != "OpenCSG/csg-wukong-1B" {
		t.Errorf("FullName() = %q, want %q", got, "OpenCSG/csg-wukong-1B")
	}
}

func TestFormat_String(t *testing.T) {
	tests := []struct {
		format Format
		want   string
	}{
		{FormatGGUF, "gguf"},
		{FormatSafeTensors, "safetensors"},
		{FormatUnknown, "unknown"},
	}

	for _, tt := range tests {
		if string(tt.format) != tt.want {
			t.Errorf("Format string = %q, want %q", string(tt.format), tt.want)
		}
	}
}
