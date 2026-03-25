package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/opencsgs/csghub-lite/internal/inference"
	"github.com/opencsgs/csghub-lite/pkg/api"
)

// POST /v1/messages -- Anthropic-compatible messages API
func (s *Server) handleAnthropicMessages(w http.ResponseWriter, r *http.Request) {
	var req api.AnthropicMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAnthropicError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Model == "" {
		writeAnthropicError(w, http.StatusBadRequest, "model is required")
		return
	}

	opts := inference.DefaultOptions()
	if req.Temperature != nil {
		opts.Temperature = *req.Temperature
	}
	if req.TopP != nil {
		opts.TopP = *req.TopP
	}
	if req.MaxTokens > 0 {
		opts.MaxTokens = req.MaxTokens
	}
	if len(req.StopSequences) > 0 {
		opts.Stop = req.StopSequences
	}

	eng, err := s.getOrLoadEngineWithNumCtx(req.Model, 0)
	if err != nil {
		writeAnthropicError(w, http.StatusBadRequest, err.Error())
		return
	}
	defer s.touchEngine(req.Model)

	messages := anthropicMessagesToInference(req)
	inputTokens := countAnthropicRequestTokens(req)
	id := fmt.Sprintf("msg_%d", time.Now().UnixNano())

	if req.Stream {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		start := api.AnthropicMessageResponse{
			ID:      id,
			Type:    "message",
			Role:    "assistant",
			Content: []api.AnthropicTextBlock{},
			Model:   req.Model,
			Usage: api.AnthropicUsage{
				InputTokens: inputTokens,
			},
		}
		writeAnthropicSSE(w, "message_start", map[string]interface{}{
			"type":    "message_start",
			"message": start,
		})
		writeAnthropicSSE(w, "content_block_start", map[string]interface{}{
			"type":  "content_block_start",
			"index": 0,
			"content_block": map[string]interface{}{
				"type": "text",
				"text": "",
			},
		})

		var full strings.Builder
		onToken := func(token string) {
			full.WriteString(token)
			writeAnthropicSSE(w, "content_block_delta", map[string]interface{}{
				"type":  "content_block_delta",
				"index": 0,
				"delta": map[string]interface{}{
					"type": "text_delta",
					"text": token,
				},
			})
		}

		if _, err := eng.Chat(r.Context(), messages, opts, onToken); err != nil {
			writeAnthropicSSE(w, "error", anthropicErrorPayload(err.Error()))
			return
		}

		outputTokens := estimateAnthropicTokens(full.String())
		writeAnthropicSSE(w, "content_block_stop", map[string]interface{}{
			"type":  "content_block_stop",
			"index": 0,
		})
		writeAnthropicSSE(w, "message_delta", map[string]interface{}{
			"type": "message_delta",
			"delta": map[string]interface{}{
				"stop_reason":   "end_turn",
				"stop_sequence": nil,
			},
			"usage": map[string]interface{}{
				"output_tokens": outputTokens,
			},
		})
		writeAnthropicSSE(w, "message_stop", map[string]interface{}{
			"type": "message_stop",
		})
		return
	}

	response, err := eng.Chat(r.Context(), messages, opts, nil)
	if err != nil {
		writeAnthropicError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, buildAnthropicMessageResponse(id, req.Model, response, inputTokens))
}

// POST /v1/messages/count_tokens -- Anthropic-compatible token counting
func (s *Server) handleAnthropicCountTokens(w http.ResponseWriter, r *http.Request) {
	var req api.AnthropicMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAnthropicError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	writeJSON(w, http.StatusOK, api.AnthropicCountTokensResponse{
		InputTokens: countAnthropicRequestTokens(req),
	})
}

func buildAnthropicMessageResponse(id, modelID, text string, inputTokens int) api.AnthropicMessageResponse {
	return api.AnthropicMessageResponse{
		ID:   id,
		Type: "message",
		Role: "assistant",
		Content: []api.AnthropicTextBlock{{
			Type: "text",
			Text: text,
		}},
		Model:        modelID,
		StopReason:   "end_turn",
		StopSequence: nil,
		Usage: api.AnthropicUsage{
			InputTokens:  inputTokens,
			OutputTokens: estimateAnthropicTokens(text),
		},
	}
}

func anthropicMessagesToInference(req api.AnthropicMessageRequest) []inference.Message {
	messages := make([]inference.Message, 0, len(req.Messages)+1)
	if system := anthropicContentText(req.System); system != "" {
		messages = append(messages, inference.Message{Role: "system", Content: system})
	}
	for _, item := range req.Messages {
		text := anthropicContentText(item.Content)
		messages = append(messages, inference.Message{
			Role:    item.Role,
			Content: text,
		})
	}
	return messages
}

func countAnthropicRequestTokens(req api.AnthropicMessageRequest) int {
	total := estimateAnthropicTokens(anthropicContentText(req.System))
	for _, item := range req.Messages {
		total += estimateAnthropicTokens(anthropicContentText(item.Content))
	}
	if total == 0 {
		return 1
	}
	return total
}

func estimateAnthropicTokens(text string) int {
	text = strings.TrimSpace(text)
	if text == "" {
		return 0
	}
	count := utf8.RuneCountInString(text) / 4
	if count < 1 {
		count = 1
	}
	return count
}

func anthropicContentText(content interface{}) string {
	switch value := content.(type) {
	case nil:
		return ""
	case string:
		return value
	case []interface{}:
		var parts []string
		for _, raw := range value {
			if part := anthropicContentText(raw); part != "" {
				parts = append(parts, part)
			}
		}
		return strings.Join(parts, "\n")
	case map[string]interface{}:
		kind, _ := value["type"].(string)
		switch kind {
		case "text":
			text, _ := value["text"].(string)
			return text
		case "tool_result":
			return anthropicContentText(value["content"])
		default:
			if text, ok := value["text"].(string); ok {
				return text
			}
			return anthropicContentText(value["content"])
		}
	default:
		data, err := json.Marshal(value)
		if err != nil {
			return ""
		}
		return string(data)
	}
}

func writeAnthropicSSE(w http.ResponseWriter, event string, payload interface{}) {
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}
	fmt.Fprintf(w, "event: %s\n", event)
	fmt.Fprintf(w, "data: %s\n\n", data)
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}

func writeAnthropicError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(anthropicErrorPayload(msg))
}

func anthropicErrorPayload(msg string) map[string]interface{} {
	return map[string]interface{}{
		"type": "error",
		"error": map[string]interface{}{
			"type":    "invalid_request_error",
			"message": msg,
		},
	}
}
