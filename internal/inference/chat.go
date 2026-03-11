package inference

import (
	"context"
)

// Session holds a conversation with context.
type Session struct {
	engine   Engine
	messages []Message
	opts     Options
}

// NewSession creates a new chat session with the given engine.
func NewSession(engine Engine, opts Options) *Session {
	return &Session{
		engine: engine,
		messages: []Message{
			{Role: "system", Content: "You are a helpful assistant."},
		},
		opts: opts,
	}
}

// Send sends a user message and returns the assistant response via streaming.
func (s *Session) Send(ctx context.Context, userMsg string, onToken TokenCallback) (string, error) {
	s.messages = append(s.messages, Message{Role: "user", Content: userMsg})

	response, err := s.engine.Chat(ctx, s.messages, s.opts, onToken)
	if err != nil {
		// Remove the failed user message
		s.messages = s.messages[:len(s.messages)-1]
		return "", err
	}

	s.messages = append(s.messages, Message{Role: "assistant", Content: response})
	return response, nil
}

// Messages returns the conversation history.
func (s *Session) Messages() []Message {
	return s.messages
}

// SetSystemPrompt sets or replaces the system prompt.
func (s *Session) SetSystemPrompt(prompt string) {
	if len(s.messages) > 0 && s.messages[0].Role == "system" {
		s.messages[0].Content = prompt
	} else {
		s.messages = append([]Message{{Role: "system", Content: prompt}}, s.messages...)
	}
}

// Engine returns the session's inference engine.
func (s *Session) Engine() Engine {
	return s.engine
}

// Options returns the session's generation options.
func (s *Session) Options() Options {
	return s.opts
}
