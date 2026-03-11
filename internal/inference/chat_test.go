package inference

import (
	"context"
	"strings"
	"testing"
)

// mockEngine implements Engine for testing.
type mockEngine struct {
	name      string
	responses []string
	callIdx   int
}

func (m *mockEngine) Generate(_ context.Context, prompt string, _ Options, onToken TokenCallback) (string, error) {
	resp := m.nextResponse()
	if onToken != nil {
		for _, word := range strings.Fields(resp) {
			onToken(word + " ")
		}
	}
	return resp, nil
}

func (m *mockEngine) Chat(_ context.Context, messages []Message, _ Options, onToken TokenCallback) (string, error) {
	resp := m.nextResponse()
	if onToken != nil {
		for _, word := range strings.Fields(resp) {
			onToken(word + " ")
		}
	}
	return resp, nil
}

func (m *mockEngine) Close() error { return nil }

func (m *mockEngine) ModelName() string { return m.name }

func (m *mockEngine) nextResponse() string {
	if m.callIdx >= len(m.responses) {
		return "default response"
	}
	resp := m.responses[m.callIdx]
	m.callIdx++
	return resp
}

func TestSession_Send(t *testing.T) {
	eng := &mockEngine{
		name:      "test/model",
		responses: []string{"Hello!", "I can help with that."},
	}

	session := NewSession(eng, DefaultOptions())

	// Check initial state (system prompt)
	msgs := session.Messages()
	if len(msgs) != 1 {
		t.Fatalf("initial messages len = %d, want 1", len(msgs))
	}
	if msgs[0].Role != "system" {
		t.Errorf("msgs[0].Role = %q, want %q", msgs[0].Role, "system")
	}

	// First message
	resp, err := session.Send(context.Background(), "Hi", nil)
	if err != nil {
		t.Fatalf("Send error: %v", err)
	}
	if resp != "Hello!" {
		t.Errorf("response = %q, want %q", resp, "Hello!")
	}

	msgs = session.Messages()
	if len(msgs) != 3 {
		t.Fatalf("messages len = %d, want 3 (system + user + assistant)", len(msgs))
	}
	if msgs[1].Role != "user" || msgs[1].Content != "Hi" {
		t.Errorf("msgs[1] = %+v, want user/Hi", msgs[1])
	}
	if msgs[2].Role != "assistant" || msgs[2].Content != "Hello!" {
		t.Errorf("msgs[2] = %+v, want assistant/Hello!", msgs[2])
	}

	// Second message
	resp2, err := session.Send(context.Background(), "Help me", nil)
	if err != nil {
		t.Fatalf("Send error: %v", err)
	}
	if resp2 != "I can help with that." {
		t.Errorf("response = %q, want %q", resp2, "I can help with that.")
	}

	msgs = session.Messages()
	if len(msgs) != 5 {
		t.Errorf("messages len = %d, want 5", len(msgs))
	}
}

func TestSession_SendWithStreaming(t *testing.T) {
	eng := &mockEngine{
		name:      "test/model",
		responses: []string{"token1 token2"},
	}

	session := NewSession(eng, DefaultOptions())

	var tokens []string
	onToken := func(token string) {
		tokens = append(tokens, token)
	}

	_, err := session.Send(context.Background(), "test", onToken)
	if err != nil {
		t.Fatalf("Send error: %v", err)
	}

	if len(tokens) == 0 {
		t.Error("no tokens received during streaming")
	}
}

func TestSession_SetSystemPrompt(t *testing.T) {
	eng := &mockEngine{name: "test/model"}
	session := NewSession(eng, DefaultOptions())

	session.SetSystemPrompt("You are a coding assistant.")

	msgs := session.Messages()
	if msgs[0].Content != "You are a coding assistant." {
		t.Errorf("system prompt = %q, want %q", msgs[0].Content, "You are a coding assistant.")
	}
}

func TestSession_Engine(t *testing.T) {
	eng := &mockEngine{name: "test/model"}
	session := NewSession(eng, DefaultOptions())

	if session.Engine().ModelName() != "test/model" {
		t.Errorf("Engine().ModelName() = %q, want %q", session.Engine().ModelName(), "test/model")
	}
}

func TestSession_Options(t *testing.T) {
	eng := &mockEngine{name: "test/model"}
	opts := Options{Temperature: 0.5, MaxTokens: 100}
	session := NewSession(eng, opts)

	got := session.Options()
	if got.Temperature != 0.5 {
		t.Errorf("Temperature = %f, want 0.5", got.Temperature)
	}
	if got.MaxTokens != 100 {
		t.Errorf("MaxTokens = %d, want 100", got.MaxTokens)
	}
}
