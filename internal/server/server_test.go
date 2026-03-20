package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/opencsgs/csghub-lite/internal/config"
	"github.com/opencsgs/csghub-lite/internal/model"
	"github.com/opencsgs/csghub-lite/pkg/api"
)

func newTestServer(t *testing.T) *Server {
	t.Helper()
	dir := t.TempDir()
	cfg := &config.Config{
		ServerURL:  "https://hub.opencsg.com",
		ListenAddr: ":0",
		ModelDir:   dir,
	}
	return New(cfg, "test")
}

func TestHandleHealth(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	s.handleHealth(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp map[string]string
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if resp["status"] != "ok" {
		t.Errorf("status = %q, want %q", resp["status"], "ok")
	}
}

func TestHandleTags_Empty(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/tags", nil)
	w := httptest.NewRecorder()

	s.handleTags(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp api.TagsResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if resp.Models != nil && len(resp.Models) != 0 {
		t.Errorf("models len = %d, want 0", len(resp.Models))
	}
}

func TestHandleTags_WithModels(t *testing.T) {
	s := newTestServer(t)

	// Create a model manifest
	lm := &model.LocalModel{
		Namespace:    "test",
		Name:         "model",
		Format:       model.FormatGGUF,
		Size:         1024,
		Files:        []string{"model.gguf"},
		DownloadedAt: time.Now(),
	}
	model.SaveManifest(s.cfg.ModelDir, lm)

	req := httptest.NewRequest(http.MethodGet, "/api/tags", nil)
	w := httptest.NewRecorder()

	s.handleTags(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp api.TagsResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if len(resp.Models) != 1 {
		t.Fatalf("models len = %d, want 1", len(resp.Models))
	}
	if resp.Models[0].Name != "test/model" {
		t.Errorf("model name = %q, want %q", resp.Models[0].Name, "test/model")
	}
}

func TestHandleShow(t *testing.T) {
	s := newTestServer(t)

	// Create a model
	lm := &model.LocalModel{
		Namespace:    "ns",
		Name:         "mdl",
		Format:       model.FormatGGUF,
		Size:         2048,
		Files:        []string{"model.gguf"},
		DownloadedAt: time.Now(),
	}
	model.SaveManifest(s.cfg.ModelDir, lm)

	body := `{"model": "ns/mdl"}`
	req := httptest.NewRequest(http.MethodPost, "/api/show", strings.NewReader(body))
	w := httptest.NewRecorder()

	s.handleShow(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp api.ShowResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Details.Name != "ns/mdl" {
		t.Errorf("details.name = %q, want %q", resp.Details.Name, "ns/mdl")
	}
}

func TestHandleShow_NotFound(t *testing.T) {
	s := newTestServer(t)

	body := `{"model": "nonexistent/model"}`
	req := httptest.NewRequest(http.MethodPost, "/api/show", strings.NewReader(body))
	w := httptest.NewRecorder()

	s.handleShow(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestHandleDelete(t *testing.T) {
	s := newTestServer(t)

	// Create a model
	lm := &model.LocalModel{
		Namespace: "ns",
		Name:      "todelete",
		Format:    model.FormatGGUF,
		Size:      100,
		Files:     []string{"model.gguf"},
	}
	model.SaveManifest(s.cfg.ModelDir, lm)

	body := `{"model": "ns/todelete"}`
	req := httptest.NewRequest(http.MethodDelete, "/api/delete", strings.NewReader(body))
	w := httptest.NewRecorder()

	s.handleDelete(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestHandleDelete_NotFound(t *testing.T) {
	s := newTestServer(t)

	body := `{"model": "nonexistent/model"}`
	req := httptest.NewRequest(http.MethodDelete, "/api/delete", strings.NewReader(body))
	w := httptest.NewRecorder()

	s.handleDelete(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestHandleGenerate_InvalidBody(t *testing.T) {
	s := newTestServer(t)

	req := httptest.NewRequest(http.MethodPost, "/api/generate", strings.NewReader("invalid json"))
	w := httptest.NewRecorder()

	s.handleGenerate(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandleChat_InvalidBody(t *testing.T) {
	s := newTestServer(t)

	req := httptest.NewRequest(http.MethodPost, "/api/chat", strings.NewReader("not json"))
	w := httptest.NewRecorder()

	s.handleChat(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandleGenerate_ModelNotFound(t *testing.T) {
	s := newTestServer(t)

	body := `{"model": "nonexistent/model", "prompt": "hello"}`
	req := httptest.NewRequest(http.MethodPost, "/api/generate", strings.NewReader(body))
	w := httptest.NewRecorder()

	s.handleGenerate(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestRoutes(t *testing.T) {
	s := newTestServer(t)
	mux := s.routes()

	tests := []struct {
		method string
		path   string
	}{
		{"GET", "/"},
		{"GET", "/api/tags"},
		{"POST", "/api/show"},
		{"POST", "/api/pull"},
		{"DELETE", "/api/delete"},
		{"POST", "/api/generate"},
		{"POST", "/api/chat"},
	}

	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)
			// Just verify no panic and some response
			if w.Code == 0 {
				t.Error("got status 0")
			}
		})
	}
}
