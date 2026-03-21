package api

import (
	"encoding/json"
	"testing"
)

func TestGenerateRequest_Serialization(t *testing.T) {
	stream := true
	req := GenerateRequest{
		Model:  "test/model",
		Prompt: "Hello, world!",
		Stream: &stream,
		Options: &ModelOptions{
			Temperature: 0.8,
			MaxTokens:   100,
		},
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var decoded GenerateRequest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if decoded.Model != req.Model {
		t.Errorf("Model = %q, want %q", decoded.Model, req.Model)
	}
	if decoded.Prompt != req.Prompt {
		t.Errorf("Prompt = %q, want %q", decoded.Prompt, req.Prompt)
	}
	if decoded.Stream == nil || *decoded.Stream != true {
		t.Error("Stream should be true")
	}
	if decoded.Options.Temperature != 0.8 {
		t.Errorf("Temperature = %f, want 0.8", decoded.Options.Temperature)
	}
}

func TestChatRequest_Serialization(t *testing.T) {
	req := ChatRequest{
		Model: "test/model",
		Messages: []Message{
			{Role: "system", Content: "You are helpful."},
			{Role: "user", Content: "Hi"},
		},
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var decoded ChatRequest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if len(decoded.Messages) != 2 {
		t.Fatalf("Messages len = %d, want 2", len(decoded.Messages))
	}
	if decoded.Messages[0].Role != "system" {
		t.Errorf("Messages[0].Role = %q, want %q", decoded.Messages[0].Role, "system")
	}
	if decoded.Messages[1].Content != "Hi" {
		t.Errorf("Messages[1].Content = %q, want %q", decoded.Messages[1].Content, "Hi")
	}
}

func TestGenerateResponse_Serialization(t *testing.T) {
	resp := GenerateResponse{
		Model:    "test/model",
		Response: "Hello!",
		Done:     true,
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var decoded GenerateResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if decoded.Model != "test/model" {
		t.Errorf("Model = %q, want %q", decoded.Model, "test/model")
	}
	if !decoded.Done {
		t.Error("Done = false, want true")
	}
}

func TestChatResponse_WithMessage(t *testing.T) {
	resp := ChatResponse{
		Model: "test/model",
		Message: &Message{
			Role:    "assistant",
			Content: "Hello!",
		},
		Done: true,
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var decoded ChatResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if decoded.Message == nil {
		t.Fatal("Message should not be nil")
	}
	if decoded.Message.Role != "assistant" {
		t.Errorf("Role = %q, want %q", decoded.Message.Role, "assistant")
	}
}

func TestPullResponse_Serialization(t *testing.T) {
	resp := PullResponse{
		Status:    "downloading model.gguf",
		Total:     1000000,
		Completed: 500000,
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var decoded PullResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if decoded.Total != 1000000 {
		t.Errorf("Total = %d, want 1000000", decoded.Total)
	}
	if decoded.Completed != 500000 {
		t.Errorf("Completed = %d, want 500000", decoded.Completed)
	}
}

func TestTagsResponse_Empty(t *testing.T) {
	resp := TagsResponse{Models: nil}
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var decoded TagsResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if decoded.Models != nil {
		t.Errorf("Models should be nil, got %v", decoded.Models)
	}
}

func TestStreamOptional(t *testing.T) {
	// When stream is not provided, it should be nil
	raw := `{"model": "test", "prompt": "hi"}`
	var req GenerateRequest
	if err := json.Unmarshal([]byte(raw), &req); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if req.Stream != nil {
		t.Error("Stream should be nil when not provided")
	}
}
