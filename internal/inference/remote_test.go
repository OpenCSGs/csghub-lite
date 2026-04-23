package inference

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/opencsgs/csghub-lite/pkg/api"
)

func TestRemoteEngineChatIncludesRequestedContextOptions(t *testing.T) {
	var got api.ChatRequest

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/chat" {
			t.Fatalf("path = %s, want /api/chat", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"message":{"role":"assistant","content":"ok"},"done":true}`)
	}))
	defer ts.Close()

	eng := NewRemoteEngine(ts.URL, "Qwen/Qwen3-0.6B-GGUF", 131072, 1)
	resp, err := eng.Chat(context.Background(), []Message{{Role: "user", Content: "hi"}}, DefaultOptions(), nil)
	if err != nil {
		t.Fatalf("Chat returned error: %v", err)
	}
	if resp != "ok" {
		t.Fatalf("resp = %q, want ok", resp)
	}
	if got.Options == nil {
		t.Fatal("options missing from request")
	}
	if got.Options.NumCtx != 131072 {
		t.Fatalf("num_ctx = %d, want 131072", got.Options.NumCtx)
	}
	if got.Options.NumParallel != 1 {
		t.Fatalf("num_parallel = %d, want 1", got.Options.NumParallel)
	}
}
