package server

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/opencsgs/csghub-lite/internal/config"
)

type zeroReader struct{}

func (zeroReader) Read(p []byte) (int, error) {
	clear(p)
	return len(p), nil
}

func TestHandleOpenAIAudioTranscriptionsUsesLiteTempDir(t *testing.T) {
	missingTempDir := filepath.Join(t.TempDir(), "missing-temp")
	t.Setenv("TMPDIR", missingTempDir)
	t.Setenv("TMP", missingTempDir)
	t.Setenv("TEMP", missingTempDir)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("model", "missing-asr-model"); err != nil {
		t.Fatalf("write model field: %v", err)
	}
	if err := writer.WriteField("response_format", "json"); err != nil {
		t.Fatalf("write response_format field: %v", err)
	}
	part, err := writer.CreateFormFile("file", "long_recording.mp3")
	if err != nil {
		t.Fatalf("create file part: %v", err)
	}
	if _, err := io.CopyN(part, zeroReader{}, maxAudioUploadMemory+1024); err != nil {
		t.Fatalf("write file part: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	storageDir := t.TempDir()
	cfg := &config.Config{
		ModelDir:   config.ModelDirForStorage(storageDir),
		DatasetDir: config.DatasetDirForStorage(storageDir),
	}
	s := New(cfg, "test")
	req := httptest.NewRequest(http.MethodPost, "/v1/audio/transcriptions", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()

	s.handleOpenAIAudioTranscriptions(w, req)

	if strings.Contains(w.Body.String(), "invalid multipart request") {
		t.Fatalf("expected upload parsing to avoid system temp dir, got status=%d body=%s", w.Code, w.Body.String())
	}
	if _, err := os.Stat(missingTempDir); !os.IsNotExist(err) {
		t.Fatalf("expected system temp dir to remain unused, stat err=%v", err)
	}
	if _, err := os.Stat(cfg.TempDir()); err != nil {
		t.Fatalf("expected lite temp dir to be used: %v", err)
	}
}

func TestHandleOpenAIAudioTranscriptionsParsesFieldsFromStreamedMultipart(t *testing.T) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("model", ""); err != nil {
		t.Fatalf("write model field: %v", err)
	}
	part, err := writer.CreateFormFile("file", "clip.mp3")
	if err != nil {
		t.Fatalf("create file part: %v", err)
	}
	if _, err := part.Write([]byte("audio")); err != nil {
		t.Fatalf("write file part: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	s := New(&config.Config{
		ModelDir:   config.ModelDirForStorage(t.TempDir()),
		DatasetDir: config.DatasetDirForStorage(t.TempDir()),
	}, "test")
	req := httptest.NewRequest(http.MethodPost, "/v1/audio/transcriptions", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()

	s.handleOpenAIAudioTranscriptions(w, req)

	if !strings.Contains(w.Body.String(), "model is required") {
		t.Fatalf("expected streamed multipart fields to be parsed, got status=%d body=%s", w.Code, w.Body.String())
	}
}
