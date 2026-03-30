package cloud

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/opencsgs/csghub-lite/pkg/api"
)

func TestModelInfoFromRemote_TextModel(t *testing.T) {
	info, ok := modelInfoFromRemote(remoteModel{
		ID:          "Qwen/Qwen3-0.6B:abc123",
		Task:        "text-generation",
		DisplayName: "Qwen3-0.6B",
		Created:     1773623409,
	})
	if !ok {
		t.Fatal("expected model to be included")
	}
	if info.Source != "cloud" {
		t.Fatalf("Source = %q, want cloud", info.Source)
	}
	if info.DisplayName != "Qwen3-0.6B" {
		t.Fatalf("DisplayName = %q, want Qwen3-0.6B", info.DisplayName)
	}
	if info.PipelineTag != "text-generation" {
		t.Fatalf("PipelineTag = %q, want text-generation", info.PipelineTag)
	}
}

func TestModelInfoFromRemote_VisionModelEnablesImages(t *testing.T) {
	info, ok := modelInfoFromRemote(remoteModel{
		ID:          "Qwen/Qwen3.5-35B-A3B-FP8:xyz",
		Task:        "image-text-to-text",
		DisplayName: "Qwen3.5-35B-A3B-FP8",
	})
	if !ok {
		t.Fatal("expected model to be included")
	}
	if !info.HasMMProj {
		t.Fatal("HasMMProj = false, want true for cloud vision models")
	}
}

func TestModelInfoFromRemote_FiltersUnsupportedTask(t *testing.T) {
	if _, ok := modelInfoFromRemote(remoteModel{
		ID:   "stabilityai/stable-diffusion-xl-base-1.0:abc",
		Task: "text-to-image",
	}); ok {
		t.Fatal("expected text-to-image model to be filtered from chat list")
	}
}

func TestModelInfoFromRemote_AllowsBlankTaskAsTextGeneration(t *testing.T) {
	info, ok := modelInfoFromRemote(remoteModel{
		ID: "claude",
	})
	if !ok {
		t.Fatal("expected blank-task model to be included")
	}
	if info.PipelineTag != "text-generation" {
		t.Fatalf("PipelineTag = %q, want text-generation fallback", info.PipelineTag)
	}
}

func TestRefreshChatModelsBypassesCache(t *testing.T) {
	requests := 0
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"object": "list",
			"data": []map[string]any{
				{
					"id":           "fresh/model",
					"task":         "text-generation",
					"display_name": "Fresh Model",
				},
			},
		})
	}))
	defer apiServer.Close()

	svc := NewService(apiServer.URL)
	svc.cached = []api.ModelInfo{{Model: "stale/model", Source: "cloud"}}
	svc.cachedAt = time.Now()

	models, err := svc.RefreshChatModels(context.Background())
	if err != nil {
		t.Fatalf("RefreshChatModels returned error: %v", err)
	}
	if requests != 1 {
		t.Fatalf("requests = %d, want 1", requests)
	}
	if len(models) != 1 || models[0].Model != "fresh/model" {
		t.Fatalf("models = %#v, want fresh/model", models)
	}

	cached, err := svc.ListChatModels(context.Background())
	if err != nil {
		t.Fatalf("ListChatModels returned error: %v", err)
	}
	if requests != 1 {
		t.Fatalf("requests after cached list = %d, want 1", requests)
	}
	if len(cached) != 1 || cached[0].Model != "fresh/model" {
		t.Fatalf("cached models = %#v, want fresh/model", cached)
	}
}
