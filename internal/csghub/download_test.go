package csghub

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestDownloadLFSFile(t *testing.T) {
	content := "hello world this is a test LFS file"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wantPath := "/api/v1/models/ns/name/resolve/config.bin"
		if r.URL.Path != wantPath {
			t.Errorf("path = %q, want %q", r.URL.Path, wantPath)
		}
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(content)))
		_, _ = w.Write([]byte(content))
	}))
	defer server.Close()

	dir := t.TempDir()
	dest := filepath.Join(dir, "config.bin")

	c := NewClient(server.URL, "")
	var progressCalled bool
	err := c.DownloadFile(context.Background(), "ns", "name", "config.bin", dest, true, int64(len(content)), "", func(downloaded, total int64) {
		progressCalled = true
	})
	if err != nil {
		t.Fatalf("DownloadFile error: %v", err)
	}

	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("ReadFile error: %v", err)
	}
	if string(data) != content {
		t.Errorf("file content = %q, want %q", string(data), content)
	}

	if !progressCalled {
		t.Error("progress callback was not called")
	}
}

func TestDownloadRawFile(t *testing.T) {
	content := "raw text content here"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wantPath := "/api/v1/models/ns/name/raw/README.md"
		if r.URL.Path != wantPath {
			t.Errorf("path = %q, want %q", r.URL.Path, wantPath)
		}
		resp := map[string]string{"msg": "OK", "data": content}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	dir := t.TempDir()
	dest := filepath.Join(dir, "README.md")

	c := NewClient(server.URL, "")
	var progressCalled bool
	err := c.DownloadFile(context.Background(), "ns", "name", "README.md", dest, false, 0, "", func(downloaded, total int64) {
		progressCalled = true
	})
	if err != nil {
		t.Fatalf("DownloadFile error: %v", err)
	}

	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("ReadFile error: %v", err)
	}
	if string(data) != content {
		t.Errorf("file content = %q, want %q", string(data), content)
	}

	if !progressCalled {
		t.Error("progress callback was not called")
	}
}

func TestDownloadLFSFile_Resume(t *testing.T) {
	fullContent := "0123456789abcdef"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rangeHeader := r.Header.Get("Range")
		if rangeHeader == "bytes=5-" {
			w.WriteHeader(http.StatusPartialContent)
			_, _ = w.Write([]byte(fullContent[5:]))
		} else {
			_, _ = w.Write([]byte(fullContent))
		}
	}))
	defer server.Close()

	dir := t.TempDir()
	dest := filepath.Join(dir, "partial.bin")

	if err := os.WriteFile(dest, []byte(fullContent[:5]), 0o644); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}

	c := NewClient(server.URL, "")
	err := c.DownloadFile(context.Background(), "ns", "name", "partial.bin", dest, true, int64(len(fullContent)), "", nil)
	if err != nil {
		t.Fatalf("DownloadFile error: %v", err)
	}

	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("ReadFile error: %v", err)
	}
	if string(data) != fullContent {
		t.Errorf("file content = %q, want %q", string(data), fullContent)
	}
}

func TestDownloadFile_CreatesDirs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]string{"msg": "OK", "data": "content"}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	dir := t.TempDir()
	dest := filepath.Join(dir, "deep", "nested", "file.txt")

	c := NewClient(server.URL, "")
	err := c.DownloadFile(context.Background(), "ns", "name", "file.txt", dest, false, 0, "", nil)
	if err != nil {
		t.Fatalf("DownloadFile error: %v", err)
	}

	if _, err := os.Stat(dest); os.IsNotExist(err) {
		t.Error("file was not created in nested directory")
	}
}

func TestDownloadLFSFile_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal error", http.StatusInternalServerError)
	}))
	defer server.Close()

	dir := t.TempDir()
	c := NewClient(server.URL, "")
	err := c.DownloadFile(context.Background(), "ns", "name", "file.bin", filepath.Join(dir, "file.bin"), true, 0, "", nil)
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}
