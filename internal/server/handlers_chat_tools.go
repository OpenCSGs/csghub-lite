package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/opencsgs/csghub-lite/internal/inference"
	"github.com/opencsgs/csghub-lite/pkg/api"
)

func hasToolChatFeatures(req api.ChatRequest) bool {
	if len(req.Tools) > 0 {
		return true
	}
	for _, msg := range req.Messages {
		if len(msg.ToolCalls) > 0 || msg.Role == "tool" || msg.ToolName != "" || msg.ToolCallID != "" {
			return true
		}
	}
	return false
}

func (s *Server) handleChatWithTools(w http.ResponseWriter, r *http.Request, req api.ChatRequest, eng inference.Engine, opts inference.Options, stream bool) {
	proxy, ok := eng.(inference.ChatCompletionProxier)
	if !ok {
		writeError(w, http.StatusBadRequest, "selected model backend does not support tool calling")
		return
	}

	openAIReq, err := ollamaChatRequestToOpenAI(req, opts)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	resp, err := proxy.ChatCompletion(r.Context(), openAIReq)
	if err != nil {
		s.writeToolChatError(w, req.Model, stream, requestWantsSSE(r), err)
		return
	}
	defer resp.Body.Close()

	var openAIResp api.OpenAIChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&openAIResp); err != nil {
		s.writeToolChatError(w, req.Model, stream, requestWantsSSE(r), fmt.Errorf("decoding tool response: %w", err))
		return
	}

	ollamaResp, err := openAIChatResponseToOllama(req.Model, openAIResp)
	if err != nil {
		s.writeToolChatError(w, req.Model, stream, requestWantsSSE(r), err)
		return
	}

	if !stream {
		ollamaResp.Done = true
		writeJSON(w, http.StatusOK, ollamaResp)
		return
	}

	sse := requestWantsSSE(r)
	if sse {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		if shouldEmitToolChunk(ollamaResp.Message) {
			writeSSE(w, ollamaResp)
		}
		writeSSE(w, api.ChatResponse{
			Model:     req.Model,
			Done:      true,
			CreatedAt: time.Now(),
		})
		return
	}

	w.Header().Set("Content-Type", "application/x-ndjson")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	if shouldEmitToolChunk(ollamaResp.Message) {
		writeNDJSON(w, ollamaResp)
	}
	writeNDJSON(w, api.ChatResponse{
		Model: req.Model,
		Message: &api.Message{
			Role:    "assistant",
			Content: "",
		},
		Done:      true,
		CreatedAt: time.Now(),
	})
}

func (s *Server) writeToolChatError(w http.ResponseWriter, model string, stream, sse bool, err error) {
	if !stream {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	resp := api.ChatResponse{
		Model: model,
		Message: &api.Message{
			Role:    "assistant",
			Content: "Error: " + err.Error(),
		},
		Done:      true,
		CreatedAt: time.Now(),
	}
	if sse {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		writeSSE(w, resp)
		return
	}
	w.Header().Set("Content-Type", "application/x-ndjson")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	writeNDJSON(w, resp)
}

func shouldEmitToolChunk(msg *api.Message) bool {
	if msg == nil {
		return false
	}
	if len(msg.ToolCalls) > 0 {
		return true
	}
	if msg.Thinking != "" {
		return true
	}
	if s, ok := msg.Content.(string); ok {
		return s != ""
	}
	return msg.Content != nil
}

func ollamaChatRequestToOpenAI(req api.ChatRequest, opts inference.Options) (map[string]interface{}, error) {
	messages, err := ollamaMessagesToOpenAI(req.Messages)
	if err != nil {
		return nil, err
	}
	body := map[string]interface{}{
		"messages":    messages,
		"temperature": opts.Temperature,
		"top_p":       opts.TopP,
		"max_tokens":  opts.MaxTokens,
		"stream":      false,
	}
	if opts.Seed >= 0 {
		body["seed"] = opts.Seed
	}
	if len(opts.Stop) > 0 {
		body["stop"] = opts.Stop
	}
	if len(req.Tools) > 0 {
		body["tools"] = req.Tools
	}
	return body, nil
}

type pendingToolCall struct {
	ID   string
	Name string
}

func ollamaMessagesToOpenAI(messages []api.Message) ([]map[string]interface{}, error) {
	out := make([]map[string]interface{}, 0, len(messages))
	pending := make([]pendingToolCall, 0)
	assistantCount := 0

	for _, msg := range messages {
		switch msg.Role {
		case "assistant":
			m := map[string]interface{}{"role": "assistant"}
			if len(msg.ToolCalls) > 0 {
				openAIToolCalls := make([]map[string]interface{}, 0, len(msg.ToolCalls))
				for idx, call := range msg.ToolCalls {
					callID := call.ID
					if callID == "" {
						callID = fmt.Sprintf("call_%d_%d", assistantCount, idx)
					}
					pending = append(pending, pendingToolCall{ID: callID, Name: call.Function.Name})
					openAIToolCalls = append(openAIToolCalls, map[string]interface{}{
						"id":   callID,
						"type": defaultToolType(call.Type),
						"function": map[string]interface{}{
							"name":      call.Function.Name,
							"arguments": toolArgumentsJSONString(call.Function.Arguments),
						},
					})
				}
				m["tool_calls"] = openAIToolCalls
				m["content"] = nil
				assistantCount++
			} else {
				m["content"] = msg.Content
			}
			out = append(out, m)
		case "tool":
			toolCallID, nextPending := matchPendingToolCall(pending, msg.ToolName)
			pending = nextPending
			m := map[string]interface{}{
				"role":    "tool",
				"content": contentAsString(msg.Content),
			}
			if toolCallID != "" {
				m["tool_call_id"] = toolCallID
			}
			if msg.ToolCallID != "" {
				m["tool_call_id"] = msg.ToolCallID
			}
			if msg.ToolName != "" {
				m["name"] = msg.ToolName
			}
			out = append(out, m)
		default:
			out = append(out, map[string]interface{}{
				"role":    msg.Role,
				"content": msg.Content,
			})
		}
	}

	return out, nil
}

func matchPendingToolCall(pending []pendingToolCall, toolName string) (string, []pendingToolCall) {
	if len(pending) == 0 {
		return "", pending
	}
	matchIdx := 0
	if toolName != "" {
		matchIdx = -1
		for i, call := range pending {
			if call.Name == toolName {
				matchIdx = i
				break
			}
		}
		if matchIdx == -1 {
			matchIdx = 0
		}
	}
	callID := pending[matchIdx].ID
	return callID, append(pending[:matchIdx], pending[matchIdx+1:]...)
}

func toolArgumentsJSONString(args interface{}) string {
	if args == nil {
		return "{}"
	}
	if s, ok := args.(string); ok {
		return s
	}
	buf, err := json.Marshal(args)
	if err != nil {
		return "{}"
	}
	return string(buf)
}

func contentAsString(content interface{}) string {
	switch v := content.(type) {
	case nil:
		return ""
	case string:
		return v
	default:
		buf, err := json.Marshal(v)
		if err != nil {
			return fmt.Sprintf("%v", v)
		}
		return string(buf)
	}
}

func openAIChatResponseToOllama(model string, resp api.OpenAIChatResponse) (api.ChatResponse, error) {
	if len(resp.Choices) == 0 || resp.Choices[0].Message == nil {
		return api.ChatResponse{}, fmt.Errorf("no choices in tool response")
	}
	msg := resp.Choices[0].Message
	out := &api.Message{
		Role:    "assistant",
		Content: contentOrEmpty(msg.Content),
	}
	if msg.Thinking != "" {
		out.Thinking = msg.Thinking
	}
	if len(msg.ToolCalls) > 0 {
		out.ToolCalls = toOllamaToolCalls(msg.ToolCalls)
	}
	return api.ChatResponse{
		Model:     model,
		Message:   out,
		Done:      false,
		CreatedAt: time.Now(),
	}, nil
}

func contentOrEmpty(content interface{}) interface{} {
	if content == nil {
		return ""
	}
	return content
}

func toOllamaToolCalls(toolCalls []api.ToolCall) []api.ToolCall {
	out := make([]api.ToolCall, 0, len(toolCalls))
	for i, call := range toolCalls {
		index := i
		out = append(out, api.ToolCall{
			ID:   call.ID,
			Type: defaultToolType(call.Type),
			Function: api.ToolFunction{
				Index:       &index,
				Name:        call.Function.Name,
				Description: call.Function.Description,
				Parameters:  call.Function.Parameters,
				Arguments:   parseToolArguments(call.Function.Arguments),
			},
		})
	}
	return out
}

func parseToolArguments(args interface{}) interface{} {
	s, ok := args.(string)
	if !ok {
		return args
	}
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return map[string]interface{}{}
	}
	var parsed interface{}
	if err := json.Unmarshal([]byte(trimmed), &parsed); err == nil {
		return parsed
	}
	return s
}

func defaultToolType(toolType string) string {
	if toolType == "" {
		return "function"
	}
	return toolType
}
