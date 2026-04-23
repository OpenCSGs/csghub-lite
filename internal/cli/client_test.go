package cli

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/opencsgs/csghub-lite/pkg/api"
)

func TestPreloadModelIncludesRequestedContextOptions(t *testing.T) {
	var got api.LoadRequest

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/load" {
			t.Fatalf("path = %s, want /api/load", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"status\":\"ready\"}\n\n")
	}))
	defer ts.Close()

	if err := preloadModel(ts.URL, "Qwen/Qwen3-0.6B-GGUF", 131072, 1); err != nil {
		t.Fatalf("preloadModel returned error: %v", err)
	}

	if got.Model != "Qwen/Qwen3-0.6B-GGUF" {
		t.Fatalf("model = %q, want Qwen/Qwen3-0.6B-GGUF", got.Model)
	}
	if got.Stream == nil || !*got.Stream {
		t.Fatalf("stream = %#v, want true", got.Stream)
	}
	if got.NumCtx != 131072 {
		t.Fatalf("num_ctx = %d, want 131072", got.NumCtx)
	}
	if got.NumParallel != 1 {
		t.Fatalf("num_parallel = %d, want 1", got.NumParallel)
	}
}
