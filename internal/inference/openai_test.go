package inference

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOpenAIEngineChatUnauthorized(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"code":"AUTH-ERR-1","msg":"AUTH-ERR-1: User not found, please login first"}`)
	}))
	defer ts.Close()

	eng := NewOpenAIEngine(ts.URL, "deepseek-v3", "test-token")
	_, err := eng.Chat(context.Background(), []Message{{Role: "user", Content: "hi"}}, DefaultOptions(), nil)
	if err == nil {
		t.Fatal("expected unauthorized error")
	}
	if got := HTTPStatusCode(err); got != http.StatusUnauthorized {
		t.Fatalf("HTTPStatusCode = %d, want %d", got, http.StatusUnauthorized)
	}
	if HTTPErrorMessage(err) != "AUTH-ERR-1: User not found, please login first" {
		t.Fatalf("error = %q", HTTPErrorMessage(err))
	}
}

func TestOpenAIEngineChatStream(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"hel\"}}]}\n\n")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"lo\"}}]}\n\n")
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer ts.Close()

	eng := NewOpenAIEngine(ts.URL, "deepseek-v3", "test-token")
	var streamed string
	got, err := eng.Chat(context.Background(), []Message{{Role: "user", Content: "hi"}}, DefaultOptions(), func(token string) {
		streamed += token
	})
	if err != nil {
		t.Fatalf("Chat returned error: %v", err)
	}
	if got != "hello" {
		t.Fatalf("got = %q, want hello", got)
	}
	if streamed != "hello" {
		t.Fatalf("streamed = %q, want hello", streamed)
	}
}

func TestOpenAIEngineChatRequestBodyMatchesCloudDefaults(t *testing.T) {
	var got map[string]interface{}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"choices":[{"message":{"content":"ok"}}]}`)
	}))
	defer ts.Close()

	eng := NewOpenAIEngine(ts.URL, "Qwen/Qwen3.5-35B-A3B-FP8:s-qwen-qwen3-5-35b-a3b-fp8-6dp9", "test-token")
	opts := DefaultOptions()
	opts.Temperature = 0.2
	opts.TopP = 0.9
	opts.MaxTokens = 200

	_, err := eng.Chat(context.Background(), []Message{
		{Role: "system", Content: "You are a helpful assistant."},
		{Role: "user", Content: "hi"},
	}, opts, func(string) {})
	if err != nil {
		t.Fatalf("Chat returned error: %v", err)
	}

	if got["model"] != "Qwen/Qwen3.5-35B-A3B-FP8:s-qwen-qwen3-5-35b-a3b-fp8-6dp9" {
		t.Fatalf("model = %v", got["model"])
	}
	if got["stream"] != true {
		t.Fatalf("stream = %v, want true", got["stream"])
	}
	if got["temperature"] != 0.2 {
		t.Fatalf("temperature = %v, want 0.2", got["temperature"])
	}
	if got["top_p"] != 0.9 {
		t.Fatalf("top_p = %v, want 0.9", got["top_p"])
	}
	if got["top_k"] != float64(10) {
		t.Fatalf("top_k = %v, want 10", got["top_k"])
	}
	if got["repetition_penalty"] != float64(1) {
		t.Fatalf("repetition_penalty = %v, want 1", got["repetition_penalty"])
	}
	if got["max_tokens"] != float64(200) {
		t.Fatalf("max_tokens = %v, want 200", got["max_tokens"])
	}
	messages, ok := got["messages"].([]interface{})
	if !ok || len(messages) != 2 {
		t.Fatalf("messages = %T %v, want 2 items", got["messages"], got["messages"])
	}
}

func TestOpenAIEngineChatRequestBodyDropsTopPForClaudeModels(t *testing.T) {
	var got map[string]interface{}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"choices":[{"message":{"content":"ok"}}]}`)
	}))
	defer ts.Close()

	eng := NewOpenAIEngine(ts.URL, "claude-opus-4-6", "test-token")
	opts := DefaultOptions()
	opts.Temperature = 0.2
	opts.TopP = 0.9

	_, err := eng.Chat(context.Background(), []Message{
		{Role: "user", Content: "hi"},
	}, opts, nil)
	if err != nil {
		t.Fatalf("Chat returned error: %v", err)
	}

	if got["temperature"] != 0.2 {
		t.Fatalf("temperature = %v, want 0.2", got["temperature"])
	}
	if _, ok := got["top_p"]; ok {
		t.Fatalf("top_p = %v, want omitted for claude models", got["top_p"])
	}
}
