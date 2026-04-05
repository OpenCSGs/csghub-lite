package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/opencsgs/csghub-lite/internal/config"
	"github.com/opencsgs/csghub-lite/pkg/api"
)

func TestHandleSettingsReturnsStorageDir(t *testing.T) {
	s := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/settings", nil)
	w := httptest.NewRecorder()

	s.handleSettings(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp api.SettingsResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	wantStorage := config.StorageDir(s.cfg.ModelDir, s.cfg.DatasetDir)
	if resp.StorageDir != wantStorage {
		t.Fatalf("storage_dir = %q, want %q", resp.StorageDir, wantStorage)
	}
	if resp.ModelDir != s.cfg.ModelDir {
		t.Fatalf("model_dir = %q, want %q", resp.ModelDir, s.cfg.ModelDir)
	}
	if resp.DatasetDir != s.cfg.DatasetDir {
		t.Fatalf("dataset_dir = %q, want %q", resp.DatasetDir, s.cfg.DatasetDir)
	}
}

func TestHandleSettingsUpdateStorageDirUpdatesModelAndDatasetDirs(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	config.Reset()

	s := newTestServer(t)
	root := filepath.Join(t.TempDir(), "shared-storage")

	body, err := json.Marshal(api.SettingsUpdateRequest{StorageDir: root})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/settings", bytes.NewReader(body))
	w := httptest.NewRecorder()

	s.handleSettingsUpdate(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp api.SettingsResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	wantModelDir := filepath.Join(root, config.ModelsDir)
	wantDatasetDir := filepath.Join(root, config.DatasetsDir)
	if resp.StorageDir != root {
		t.Fatalf("storage_dir = %q, want %q", resp.StorageDir, root)
	}
	if resp.ModelDir != wantModelDir {
		t.Fatalf("model_dir = %q, want %q", resp.ModelDir, wantModelDir)
	}
	if resp.DatasetDir != wantDatasetDir {
		t.Fatalf("dataset_dir = %q, want %q", resp.DatasetDir, wantDatasetDir)
	}

	if _, err := os.Stat(wantModelDir); err != nil {
		t.Fatalf("model dir not created: %v", err)
	}
	if _, err := os.Stat(wantDatasetDir); err != nil {
		t.Fatalf("dataset dir not created: %v", err)
	}
}
