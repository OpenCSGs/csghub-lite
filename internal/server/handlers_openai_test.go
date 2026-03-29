package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/opencsgs/csghub-lite/internal/cloud"
	"github.com/opencsgs/csghub-lite/internal/config"
	"github.com/opencsgs/csghub-lite/internal/inference"
	"github.com/opencsgs/csghub-lite/internal/model"
	"github.com/opencsgs/csghub-lite/pkg/api"
)

type fakeChatCompletionEngine struct {
	resp    api.OpenAIChatResponse
	lastReq map[string]interface{}
}

func (e *fakeChatCompletionEngine) Generate(context.Context, string, inference.Options, inference.TokenCallback) (string, error) {
	return "", nil
}

func (e *fakeChatCompletionEngine) Chat(context.Context, []inference.Message, inference.Options, inference.TokenCallback) (string, error) {
	return "", nil
}

func (e *fakeChatCompletionEngine) Close() error { return nil }

func (e *fakeChatCompletionEngine) ModelName() string { return "test/model" }

func (e *fakeChatCompletionEngine) ChatCompletion(_ context.Context, reqBody map[string]interface{}) (*http.Response, error) {
	e.lastReq = reqBody
	data, err := json.Marshal(e.resp)
	if err != nil {
		return nil, err
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewReader(data)),
		Header:     make(http.Header),
	}, nil
}

func newCloudOpenAIAPIServer(t *testing.T, expectedToken string) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/models":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"object": "list",
				"data": []map[string]any{
					{
						"id":           "cloud/model",
						"object":       "model",
						"created":      456,
						"owned_by":     "opencsg",
						"task":         "text-generation",
						"display_name": "Cloud Model",
						"public":       true,
					},
				},
			})
		case "/v1/chat/completions":
			if expectedToken != "" {
				if got := r.Header.Get("Authorization"); got != "Bearer "+expectedToken {
					t.Fatalf("Authorization = %q, want %q", got, "Bearer "+expectedToken)
				}
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(api.OpenAIChatResponse{
				ID:      "chatcmpl-cloud",
				Object:  "chat.completion",
				Created: 789,
				Model:   "cloud/model",
				Choices: []api.OpenAIChoice{{
					Index: 0,
					Message: &api.Message{
						Role:    "assistant",
						Content: "cloud reply",
					},
				}},
			})
		default:
			http.NotFound(w, r)
		}
	}))
}

func TestNormalizeOpenAIToolResponseFromJSONText(t *testing.T) {
	tools := []api.Tool{{
		Type: "function",
		Function: api.ToolFunction{
			Name:       "get_weather",
			Parameters: map[string]interface{}{"type": "object", "required": []interface{}{"city"}},
		},
	}}
	resp := api.OpenAIChatResponse{
		Choices: []api.OpenAIChoice{{
			Message: &api.Message{
				Role:    "assistant",
				Content: "{\"name\":\"get_weather\",\"arguments\":{\"city\":\"Beijing\"}}",
			},
		}},
	}

	got := normalizeOpenAIToolResponse(resp, tools)
	if got.Choices[0].Message == nil || len(got.Choices[0].Message.ToolCalls) != 1 {
		t.Fatalf("expected synthesized tool call, got %#v", got.Choices[0].Message)
	}
	call := got.Choices[0].Message.ToolCalls[0]
	if call.Function.Name != "get_weather" {
		t.Fatalf("unexpected tool name: %#v", call.Function.Name)
	}
	if call.Function.Arguments != "{\"city\":\"Beijing\"}" {
		t.Fatalf("unexpected arguments payload: %#v", call.Function.Arguments)
	}
	if got.Choices[0].Message.Content != nil {
		t.Fatalf("expected response content to be cleared, got %#v", got.Choices[0].Message.Content)
	}
	if got.Choices[0].FinishReason == nil || *got.Choices[0].FinishReason != "tool_calls" {
		t.Fatalf("unexpected finish reason: %#v", got.Choices[0].FinishReason)
	}
}

func TestNormalizeOpenAIToolResponseFromBareToolName(t *testing.T) {
	tools := []api.Tool{{
		Type: "function",
		Function: api.ToolFunction{
			Name:       "get_time",
			Parameters: map[string]interface{}{"type": "object", "properties": map[string]interface{}{}},
		},
	}}
	resp := api.OpenAIChatResponse{
		Choices: []api.OpenAIChoice{{
			Message: &api.Message{
				Role:    "assistant",
				Content: "get_time",
			},
		}},
	}

	got := normalizeOpenAIToolResponse(resp, tools)
	if got.Choices[0].Message == nil || len(got.Choices[0].Message.ToolCalls) != 1 {
		t.Fatalf("expected synthesized tool call, got %#v", got.Choices[0].Message)
	}
	call := got.Choices[0].Message.ToolCalls[0]
	if call.Function.Name != "get_time" {
		t.Fatalf("unexpected tool name: %#v", call.Function.Name)
	}
	if call.Function.Arguments != "{}" {
		t.Fatalf("unexpected arguments payload: %#v", call.Function.Arguments)
	}
}

func TestHandleOpenAIChatCompletionsWithToolsSynthesizesToolCalls(t *testing.T) {
	engine := &fakeChatCompletionEngine{
		resp: api.OpenAIChatResponse{
			ID:      "chatcmpl-test",
			Object:  "chat.completion",
			Created: 123,
			Model:   "test/model",
			Choices: []api.OpenAIChoice{{
				Index: 0,
				Message: &api.Message{
					Role:    "assistant",
					Content: "get_time",
				},
			}},
		},
	}
	cfg := &config.Config{ModelDir: t.TempDir()}
	if err := model.SaveManifest(cfg.ModelDir, &model.LocalModel{
		Namespace:    "test",
		Name:         "model",
		Format:       model.FormatGGUF,
		Size:         1,
		Files:        []string{"model.gguf", "config.json"},
		DownloadedAt: time.Now(),
	}); err != nil {
		t.Fatalf("save model manifest: %v", err)
	}
	modelDir := filepath.Join(cfg.ModelDir, "test", "model")
	if err := os.MkdirAll(modelDir, 0o755); err != nil {
		t.Fatalf("mkdir model dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(modelDir, "config.json"), []byte(`{"max_position_embeddings":40960}`), 0o644); err != nil {
		t.Fatalf("write config.json: %v", err)
	}

	s := New(cfg, "test")
	s.engines["test/model"] = &managedEngine{engine: engine, numCtx: 16384}

	body := `{
	  "model": "test/model",
	  "messages": [{"role":"user","content":"Call get_time if a tool is available."}],
	  "tools": [{
	    "type":"function",
	    "function":{
	      "name":"get_time",
	      "description":"Get current time",
	      "parameters":{"type":"object","properties":{}}
	    }
	  }],
	  "tool_choice":"auto",
	  "parallel_tool_calls": false,
	  "stream": false
	}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	w := httptest.NewRecorder()

	s.handleOpenAIChatCompletions(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("unexpected status code: %d body=%s", w.Code, w.Body.String())
	}

	var resp api.OpenAIChatResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.Choices) != 1 || resp.Choices[0].Message == nil {
		t.Fatalf("unexpected choices payload: %#v", resp.Choices)
	}
	if len(resp.Choices[0].Message.ToolCalls) != 1 {
		t.Fatalf("expected one tool call, got %#v", resp.Choices[0].Message.ToolCalls)
	}
	if resp.Choices[0].Message.ToolCalls[0].Function.Name != "get_time" {
		t.Fatalf("unexpected tool call: %#v", resp.Choices[0].Message.ToolCalls[0])
	}
	if resp.Choices[0].FinishReason == nil || *resp.Choices[0].FinishReason != "tool_calls" {
		t.Fatalf("unexpected finish reason: %#v", resp.Choices[0].FinishReason)
	}

	if engine.lastReq["tool_choice"] != "auto" {
		t.Fatalf("tool_choice was not forwarded: %#v", engine.lastReq["tool_choice"])
	}
	if engine.lastReq["parallel_tool_calls"] != false {
		t.Fatalf("parallel_tool_calls was not forwarded: %#v", engine.lastReq["parallel_tool_calls"])
	}
	if engine.lastReq["stream"] != false {
		t.Fatalf("expected upstream tool request to disable streaming, got %#v", engine.lastReq["stream"])
	}
	tools, ok := engine.lastReq["tools"].([]api.Tool)
	if !ok || len(tools) != 1 {
		t.Fatalf("expected forwarded tools, got %#v", engine.lastReq["tools"])
	}
}

func TestHandleOpenAIModelsUsesCsghubOwner(t *testing.T) {
	cfg := &config.Config{ModelDir: t.TempDir()}
	if err := model.SaveManifest(cfg.ModelDir, &model.LocalModel{
		Namespace:    "Qwen",
		Name:         "Qwen3.5-2B",
		Format:       model.FormatGGUF,
		Size:         4_000_000_000,
		Files:        []string{"model.gguf"},
		DownloadedAt: time.Unix(123, 0),
	}); err != nil {
		t.Fatalf("save model manifest: %v", err)
	}

	s := &Server{
		cfg:     cfg,
		manager: model.NewManager(cfg),
	}
	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	w := httptest.NewRecorder()

	s.handleOpenAIModels(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("unexpected status code: %d body=%s", w.Code, w.Body.String())
	}

	var resp api.OpenAIModelList
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.Data) != 1 {
		t.Fatalf("expected one model, got %#v", resp.Data)
	}
	if resp.Data[0].ID != "Qwen/Qwen3.5-2B" {
		t.Fatalf("unexpected model id: %#v", resp.Data[0].ID)
	}
	if resp.Data[0].OwnedBy != "csghub" {
		t.Fatalf("unexpected owner: %#v", resp.Data[0].OwnedBy)
	}
}

func TestHandleOpenAIModelsIncludesCloudModels(t *testing.T) {
	cfg := &config.Config{ModelDir: t.TempDir(), Token: "test-token"}
	if err := model.SaveManifest(cfg.ModelDir, &model.LocalModel{
		Namespace:    "Qwen",
		Name:         "Qwen3.5-2B",
		Format:       model.FormatGGUF,
		Size:         4_000_000_000,
		Files:        []string{"model.gguf"},
		DownloadedAt: time.Unix(123, 0),
	}); err != nil {
		t.Fatalf("save model manifest: %v", err)
	}

	apiServer := newCloudOpenAIAPIServer(t, "test-token")
	defer apiServer.Close()

	s := &Server{
		cfg:     cfg,
		manager: model.NewManager(cfg),
		cloud:   cloud.NewService(apiServer.URL),
	}
	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	w := httptest.NewRecorder()

	s.handleOpenAIModels(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("unexpected status code: %d body=%s", w.Code, w.Body.String())
	}

	var resp api.OpenAIModelList
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.Data) != 2 {
		t.Fatalf("expected two models, got %#v", resp.Data)
	}
	if resp.Data[0].ID != "Qwen/Qwen3.5-2B" {
		t.Fatalf("first model id = %q, want local model", resp.Data[0].ID)
	}
	if resp.Data[1].ID != "cloud/model" {
		t.Fatalf("second model id = %q, want cloud/model", resp.Data[1].ID)
	}
}

func TestHandleModelsAnthropicFormatIncludesCloudModels(t *testing.T) {
	cfg := &config.Config{ModelDir: t.TempDir(), Token: "test-token"}
	if err := model.SaveManifest(cfg.ModelDir, &model.LocalModel{
		Namespace:    "Qwen",
		Name:         "Qwen3.5-2B",
		Format:       model.FormatGGUF,
		Size:         4_000_000_000,
		Files:        []string{"model.gguf"},
		DownloadedAt: time.Unix(123, 0),
	}); err != nil {
		t.Fatalf("save model manifest: %v", err)
	}

	apiServer := newCloudOpenAIAPIServer(t, "test-token")
	defer apiServer.Close()

	s := &Server{
		cfg:     cfg,
		manager: model.NewManager(cfg),
		cloud:   cloud.NewService(apiServer.URL),
	}
	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	req.Header.Set("Anthropic-Version", "2023-06-01")
	w := httptest.NewRecorder()

	s.handleModels(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("unexpected status code: %d body=%s", w.Code, w.Body.String())
	}

	var resp api.AnthropicModelListResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.Data) != 2 {
		t.Fatalf("expected two models, got %#v", resp.Data)
	}
	if resp.Data[0].ID != "Qwen/Qwen3.5-2B" {
		t.Fatalf("first model id = %q, want local model", resp.Data[0].ID)
	}
	if resp.Data[1].ID != "cloud/model" {
		t.Fatalf("second model id = %q, want cloud/model", resp.Data[1].ID)
	}
	if resp.Data[1].Type != "model" {
		t.Fatalf("cloud model type = %q, want model", resp.Data[1].Type)
	}
	if resp.FirstID != "Qwen/Qwen3.5-2B" || resp.LastID != "cloud/model" {
		t.Fatalf("unexpected pagination metadata: %#v", resp)
	}
}

func TestHandleAnthropicMessagesSupportsCloudModels(t *testing.T) {
	cfg := &config.Config{ModelDir: t.TempDir(), Token: "test-token"}
	apiServer := newCloudOpenAIAPIServer(t, "test-token")
	defer apiServer.Close()

	s := New(cfg, "test")
	s.cloud = cloud.NewService(apiServer.URL)

	body := `{"model":"cloud/model","messages":[{"role":"user","content":"hi"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(body))
	req.Header.Set("Anthropic-Version", "2023-06-01")
	w := httptest.NewRecorder()

	s.handleAnthropicMessages(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", w.Code, http.StatusOK, w.Body.String())
	}

	var resp api.AnthropicMessageResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Model != "cloud/model" {
		t.Fatalf("model = %q, want cloud/model", resp.Model)
	}
	if len(resp.Content) != 1 || resp.Content[0].Text != "cloud reply" {
		t.Fatalf("unexpected content: %#v", resp.Content)
	}
}

func TestHandleOpenAIChatCompletionsCloudWithoutTokenReturnsUnauthorized(t *testing.T) {
	s := newTestServer(t)
	apiServer := newCloudOpenAIAPIServer(t, "")
	defer apiServer.Close()
	s.cloud = cloud.NewService(apiServer.URL)

	body := `{"model":"cloud/model","messages":[{"role":"user","content":"hi"}],"stream":false}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	w := httptest.NewRecorder()

	s.handleOpenAIChatCompletions(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d body=%s", w.Code, http.StatusUnauthorized, w.Body.String())
	}

	var resp struct {
		Error struct {
			Message string `json:"message"`
			Type    string `json:"type"`
		} `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Error.Type != "authentication_error" {
		t.Fatalf("error type = %q, want authentication_error", resp.Error.Type)
	}
	if !strings.Contains(resp.Error.Message, "Cloud login required") {
		t.Fatalf("error message = %q, want Cloud login required", resp.Error.Message)
	}
}

func TestHandleOpenAIResponsesCloudWithoutTokenReturnsUnauthorized(t *testing.T) {
	s := newTestServer(t)
	apiServer := newCloudOpenAIAPIServer(t, "")
	defer apiServer.Close()
	s.cloud = cloud.NewService(apiServer.URL)

	body := `{"model":"cloud/model","input":"hello"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(body))
	w := httptest.NewRecorder()

	s.handleOpenAIResponses(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d body=%s", w.Code, http.StatusUnauthorized, w.Body.String())
	}

	var resp struct {
		Error struct {
			Message string `json:"message"`
			Type    string `json:"type"`
		} `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Error.Type != "authentication_error" {
		t.Fatalf("error type = %q, want authentication_error", resp.Error.Type)
	}
	if !strings.Contains(resp.Error.Message, "Cloud login required") {
		t.Fatalf("error message = %q, want Cloud login required", resp.Error.Message)
	}
}
