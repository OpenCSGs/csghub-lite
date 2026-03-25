package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/opencsgs/csghub-lite/internal/inference"
)

type openAIResponsesRequest struct {
	Model           string      `json:"model"`
	Input           interface{} `json:"input"`
	Instructions    string      `json:"instructions,omitempty"`
	Stream          bool        `json:"stream,omitempty"`
	MaxOutputTokens *int        `json:"max_output_tokens,omitempty"`
	Temperature     *float64    `json:"temperature,omitempty"`
	TopP            *float64    `json:"top_p,omitempty"`
}

// POST /v1/responses -- minimal OpenAI Responses API compatibility for Codex/OpenAI SDK clients.
func (s *Server) handleOpenAIResponses(w http.ResponseWriter, r *http.Request) {
	var req openAIResponsesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeOpenAIError(w, http.StatusBadRequest, "invalid_request_error", "invalid request body")
		return
	}
	if req.Model == "" {
		writeOpenAIError(w, http.StatusBadRequest, "invalid_request_error", "model is required")
		return
	}

	opts := inference.DefaultOptions()
	if req.Temperature != nil {
		opts.Temperature = *req.Temperature
	}
	if req.TopP != nil {
		opts.TopP = *req.TopP
	}
	if req.MaxOutputTokens != nil && *req.MaxOutputTokens > 0 {
		opts.MaxTokens = *req.MaxOutputTokens
	}

	eng, err := s.getOrLoadEngineWithNumCtx(req.Model, 0)
	if err != nil {
		writeOpenAIError(w, http.StatusBadRequest, "model_not_found", err.Error())
		return
	}
	defer s.touchEngine(req.Model)

	messages := responsesRequestMessages(req)
	inputTokens := countResponsesTokens(req)
	id := fmt.Sprintf("resp_%d", time.Now().UnixNano())
	itemID := fmt.Sprintf("msg_%d", time.Now().UnixNano())
	created := time.Now().Unix()

	if req.Stream {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		writeResponsesSSE(w, "response.created", map[string]interface{}{
			"type":     "response.created",
			"response": buildResponsesResponse(id, itemID, req.Model, "", created, "in_progress", inputTokens),
		})
		writeResponsesSSE(w, "response.output_item.added", map[string]interface{}{
			"type":         "response.output_item.added",
			"output_index": 0,
			"item":         buildResponsesOutputItem(itemID, "", "in_progress"),
		})
		writeResponsesSSE(w, "response.content_part.added", map[string]interface{}{
			"type":          "response.content_part.added",
			"output_index":  0,
			"content_index": 0,
			"item_id":       itemID,
			"part": map[string]interface{}{
				"type":        "output_text",
				"text":        "",
				"annotations": []interface{}{},
			},
		})

		var full strings.Builder
		onToken := func(token string) {
			full.WriteString(token)
			writeResponsesSSE(w, "response.output_text.delta", map[string]interface{}{
				"type":          "response.output_text.delta",
				"output_index":  0,
				"content_index": 0,
				"item_id":       itemID,
				"delta":         token,
			})
		}

		if _, err := eng.Chat(r.Context(), messages, opts, onToken); err != nil {
			writeResponsesSSE(w, "error", map[string]interface{}{
				"type":    "error",
				"message": err.Error(),
			})
			fmt.Fprintf(w, "event: done\ndata: [DONE]\n\n")
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
			return
		}

		text := full.String()
		writeResponsesSSE(w, "response.output_text.done", map[string]interface{}{
			"type":          "response.output_text.done",
			"output_index":  0,
			"content_index": 0,
			"item_id":       itemID,
			"text":          text,
		})
		writeResponsesSSE(w, "response.content_part.done", map[string]interface{}{
			"type":          "response.content_part.done",
			"output_index":  0,
			"content_index": 0,
			"item_id":       itemID,
			"part": map[string]interface{}{
				"type":        "output_text",
				"text":        text,
				"annotations": []interface{}{},
			},
		})
		writeResponsesSSE(w, "response.output_item.done", map[string]interface{}{
			"type":         "response.output_item.done",
			"output_index": 0,
			"item":         buildResponsesOutputItem(itemID, text, "completed"),
		})
		writeResponsesSSE(w, "response.completed", map[string]interface{}{
			"type":     "response.completed",
			"response": buildResponsesResponse(id, itemID, req.Model, text, created, "completed", inputTokens),
		})
		fmt.Fprintf(w, "event: done\ndata: [DONE]\n\n")
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		return
	}

	text, err := eng.Chat(r.Context(), messages, opts, nil)
	if err != nil {
		writeOpenAIError(w, http.StatusInternalServerError, "server_error", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, buildResponsesResponse(id, itemID, req.Model, text, created, "completed", inputTokens))
}

// GET /v1/responses -- return a clear error instead of the web UI index for websocket probes.
func (s *Server) handleOpenAIResponsesUnsupported(w http.ResponseWriter, r *http.Request) {
	writeOpenAIError(w, http.StatusUpgradeRequired, "invalid_request_error", "websocket transport is not supported on this endpoint")
}

func responsesRequestMessages(req openAIResponsesRequest) []inference.Message {
	rawMessages := responsesInputToMessages(req.Input)
	systemParts := make([]string, 0, 2)
	if instructions := strings.TrimSpace(req.Instructions); instructions != "" {
		systemParts = append(systemParts, instructions)
	}

	messages := make([]inference.Message, 0, len(rawMessages)+1)
	for _, message := range rawMessages {
		text := strings.TrimSpace(fmt.Sprint(message.Content))
		switch message.Role {
		case "system", "developer":
			if text != "" {
				systemParts = append(systemParts, text)
			}
		default:
			if message.Role == "" {
				message.Role = "user"
			}
			messages = append(messages, message)
		}
	}

	if len(systemParts) > 0 {
		messages = append([]inference.Message{{
			Role:    "system",
			Content: strings.Join(systemParts, "\n\n"),
		}}, messages...)
	}
	if len(messages) == 0 {
		messages = append(messages, inference.Message{Role: "user", Content: ""})
	}
	return messages
}

func responsesInputToMessages(input interface{}) []inference.Message {
	switch value := input.(type) {
	case nil:
		return nil
	case string:
		return []inference.Message{{Role: "user", Content: value}}
	case []interface{}:
		var messages []inference.Message
		for _, item := range value {
			messages = append(messages, responsesInputToMessages(item)...)
		}
		return messages
	case map[string]interface{}:
		role, _ := value["role"].(string)
		kind, _ := value["type"].(string)
		switch {
		case role != "":
			return []inference.Message{{
				Role:    role,
				Content: responsesContentText(value["content"]),
			}}
		case kind == "message":
			return []inference.Message{{
				Role:    "user",
				Content: responsesContentText(value["content"]),
			}}
		case kind == "input_text" || kind == "output_text" || kind == "text":
			return []inference.Message{{
				Role:    "user",
				Content: responsesContentText(value),
			}}
		default:
			text := responsesContentText(value)
			if text == "" {
				return nil
			}
			return []inference.Message{{Role: "user", Content: text}}
		}
	default:
		return []inference.Message{{Role: "user", Content: responsesContentText(value)}}
	}
}

func responsesContentText(content interface{}) string {
	switch value := content.(type) {
	case nil:
		return ""
	case string:
		return value
	case []interface{}:
		var parts []string
		for _, item := range value {
			if text := responsesContentText(item); text != "" {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, "\n")
	case map[string]interface{}:
		if text, ok := value["text"].(string); ok {
			return text
		}
		if inputText, ok := value["input_text"].(string); ok {
			return inputText
		}
		return responsesContentText(value["content"])
	default:
		data, err := json.Marshal(value)
		if err != nil {
			return ""
		}
		return string(data)
	}
}

func countResponsesTokens(req openAIResponsesRequest) int {
	total := estimateAnthropicTokens(req.Instructions)
	total += estimateAnthropicTokens(responsesContentText(req.Input))
	if total == 0 {
		return 1
	}
	return total
}

func buildResponsesOutputItem(itemID, text, status string) map[string]interface{} {
	return map[string]interface{}{
		"id":     itemID,
		"type":   "message",
		"status": status,
		"role":   "assistant",
		"content": []map[string]interface{}{{
			"type":        "output_text",
			"text":        text,
			"annotations": []interface{}{},
		}},
	}
}

func buildResponsesResponse(id, itemID, modelID, text string, created int64, status string, inputTokens int) map[string]interface{} {
	outputTokens := estimateAnthropicTokens(text)
	return map[string]interface{}{
		"id":                  id,
		"object":              "response",
		"created_at":          created,
		"status":              status,
		"model":               modelID,
		"output":              []interface{}{buildResponsesOutputItem(itemID, text, status)},
		"output_text":         text,
		"parallel_tool_calls": false,
		"usage": map[string]interface{}{
			"input_tokens":  inputTokens,
			"output_tokens": outputTokens,
			"total_tokens":  inputTokens + outputTokens,
		},
	}
}

func writeResponsesSSE(w http.ResponseWriter, event string, payload interface{}) {
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
