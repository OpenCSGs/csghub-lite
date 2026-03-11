package csghub

import (
	"testing"
)

func TestParseModelID(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantNS    string
		wantName  string
		wantError bool
	}{
		{
			name:     "valid model ID",
			input:    "OpenCSG/csg-wukong-1B",
			wantNS:   "OpenCSG",
			wantName: "csg-wukong-1B",
		},
		{
			name:     "valid with dots and hyphens",
			input:    "my-org/my.model-v2",
			wantNS:   "my-org",
			wantName: "my.model-v2",
		},
		{
			name:      "no slash",
			input:     "justname",
			wantError: true,
		},
		{
			name:      "empty namespace",
			input:     "/name",
			wantError: true,
		},
		{
			name:      "empty name",
			input:     "namespace/",
			wantError: true,
		},
		{
			name:      "empty string",
			input:     "",
			wantError: true,
		},
		{
			name:     "multiple slashes (takes first)",
			input:    "ns/name/extra",
			wantNS:   "ns",
			wantName: "name/extra",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ns, name, err := ParseModelID(tt.input)
			if tt.wantError {
				if err == nil {
					t.Errorf("ParseModelID(%q) = (%q, %q, nil), want error", tt.input, ns, name)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseModelID(%q) error: %v", tt.input, err)
			}
			if ns != tt.wantNS {
				t.Errorf("namespace = %q, want %q", ns, tt.wantNS)
			}
			if name != tt.wantName {
				t.Errorf("name = %q, want %q", name, tt.wantName)
			}
		})
	}
}
