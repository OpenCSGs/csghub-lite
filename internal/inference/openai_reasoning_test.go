package inference

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMessagesToOpenAIPreservesReasoningContent(t *testing.T) {
	messages := []Message{
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "hi", ReasoningContent: "thinking about the response"},
		{Role: "user", Content: "thanks"},
	}

	out := messagesToOpenAI(messages)

	if len(out) != 3 {
		t.Fatalf("len(out) = %d, want 3", len(out))
	}

	// First message (user) should not have reasoning_content
	if _, ok := out[0]["reasoning_content"]; ok {
		t.Errorf("out[0] has reasoning_content, want none for user message")
	}

	// Second message (assistant) should have reasoning_content
	if rc, ok := out[1]["reasoning_content"].(string); !ok || rc != "thinking about the response" {
		t.Errorf("out[1][\"reasoning_content\"] = %v, want \"thinking about the response\"", out[1]["reasoning_content"])
	}

	// Third message (user) should not have reasoning_content
	if _, ok := out[2]["reasoning_content"]; ok {
		t.Errorf("out[2] has reasoning_content, want none for user message")
	}
}

func TestMessagesToOpenAISkipsEmptyReasoningContent(t *testing.T) {
	messages := []Message{
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "hi", ReasoningContent: ""}, // empty
		{Role: "user", Content: "thanks"},
	}

	out := messagesToOpenAI(messages)

	if len(out) != 3 {
		t.Fatalf("len(out) = %d, want 3", len(out))
	}

	// Assistant message with empty ReasoningContent should not have the field
	if _, ok := out[1]["reasoning_content"]; ok {
		t.Errorf("out[1] has reasoning_content, want none when empty")
	}
}

func TestOpenAIEngineChatPassesReasoningContentToAPI(t *testing.T) {
	var receivedBody map[string]interface{}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&receivedBody); err != nil {
			t.Errorf("decoding request body: %v", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		// Return a simple response
		fmt.Fprint(w, `{"choices":[{"message":{"content":"ok"}}]}`)
	}))
	defer ts.Close()

	eng := NewOpenAIEngine(ts.URL, "deepseek-v4-pro", "test-token")
	messages := []Message{
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "hi", ReasoningContent: "I thought about this"},
		{Role: "user", Content: "continue"},
	}

	_, err := eng.Chat(context.Background(), messages, DefaultOptions(), nil)
	if err != nil {
		t.Fatalf("Chat returned error: %v", err)
	}

	// Verify the request body contains reasoning_content
	reqMessages, ok := receivedBody["messages"].([]interface{})
	if !ok {
		t.Fatalf("messages in request body is not an array: %T", receivedBody["messages"])
	}

	if len(reqMessages) != 3 {
		t.Fatalf("len(messages) = %d, want 3", len(reqMessages))
	}

	// Check assistant message has reasoning_content
	assistantMsg, ok := reqMessages[1].(map[string]interface{})
	if !ok {
		t.Fatalf("assistant message is not a map: %T", reqMessages[1])
	}

	if rc, ok := assistantMsg["reasoning_content"].(string); !ok || rc != "I thought about this" {
		t.Errorf("assistant message reasoning_content = %v, want \"I thought about this\"", assistantMsg["reasoning_content"])
	}
}
