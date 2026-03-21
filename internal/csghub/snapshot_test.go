package csghub

import (
	"reflect"
	"testing"
)

func TestFilterGGUFMultiQuantDownload(t *testing.T) {
	files := []RepoFile{
		{Type: "file", Path: "README.md", Name: "README.md"},
		{Type: "file", Path: "Q8_0.gguf", Name: "Q8_0.gguf", LFS: true},
		{Type: "file", Path: "Q4_0.gguf", Name: "Q4_0.gguf", LFS: true},
	}
	got := filterGGUFMultiQuantDownload(files)
	var names []string
	for _, f := range got {
		names = append(names, f.Name)
	}
	want := []string{"README.md", "Q8_0.gguf"}
	if !reflect.DeepEqual(names, want) {
		t.Errorf("got %v, want %v", names, want)
	}
}

func TestFilterGGUFMultiQuantDownload_singleGGUF(t *testing.T) {
	files := []RepoFile{
		{Type: "file", Path: "Q4_0.gguf", Name: "Q4_0.gguf"},
	}
	got := filterGGUFMultiQuantDownload(files)
	if len(got) != 1 {
		t.Fatalf("len = %d", len(got))
	}
}

func TestFilterGGUFMultiQuantDownload_nestedQuantDirs(t *testing.T) {
	files := []RepoFile{
		{Type: "file", Path: "README.md", Name: "README.md"},
		{Type: "file", Path: "Q4_0/model.gguf", Name: "model.gguf", LFS: true},
		{Type: "file", Path: "Q8_0/model.gguf", Name: "model.gguf", LFS: true},
	}
	got := filterGGUFMultiQuantDownload(files)
	var paths []string
	for _, f := range got {
		paths = append(paths, f.Path)
	}
	want := []string{"README.md", "Q8_0/model.gguf"}
	if !reflect.DeepEqual(paths, want) {
		t.Errorf("got %v, want %v", paths, want)
	}
}

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
