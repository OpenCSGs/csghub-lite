package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/opencsgs/csghub-lite/internal/inference"
	"github.com/opencsgs/csghub-lite/internal/websearch"
	"github.com/opencsgs/csghub-lite/pkg/api"
)

const webSearchToolName = "web_search"

var webSearchToolDef = api.Tool{
	Type: "function",
	Function: api.ToolFunction{
		Name:        webSearchToolName,
		Description: "Search the web for current information. Use this when the user asks about recent events, real-time data, or facts you are unsure about.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"query": map[string]interface{}{
					"type":        "string",
					"description": "The search query",
				},
			},
			"required": []string{"query"},
		},
	},
}

const webSearchSystemSuffix = "\n\nYou have access to a web_search tool. Use it when the user asks about current events, real-time data, recent news, or information you are unsure about. Do not use it for general knowledge questions you can answer confidently."

func (s *Server) shouldInjectWebSearch(eng inference.Engine) bool {
	_, ok := eng.(inference.ChatCompletionProxier)
	return ok
}

func (s *Server) handleChatWithWebSearch(w http.ResponseWriter, r *http.Request, req api.ChatRequest, eng inference.Engine, opts inference.Options, stream bool) {
	proxy := eng.(inference.ChatCompletionProxier)

	searchReq := req
	searchReq.Tools = append([]api.Tool{webSearchToolDef}, searchReq.Tools...)

	if !hasSystemMessage(searchReq.Messages) {
		searchReq.Messages = append([]api.Message{{
			Role:    "system",
			Content: strings.TrimSpace(webSearchSystemSuffix),
		}}, searchReq.Messages...)
	} else {
		searchReq.Messages = appendToSystemMessage(searchReq.Messages, webSearchSystemSuffix)
	}

	openAIReq, err := ollamaChatRequestToOpenAI(searchReq, opts)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	openAIReq["stream"] = false

	resp, err := proxy.ChatCompletion(r.Context(), openAIReq)
	if err != nil {
		s.writeToolChatError(w, req.Model, stream, requestWantsSSE(r), err)
		return
	}
	defer resp.Body.Close()

	var firstResp api.OpenAIChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&firstResp); err != nil {
		s.writeToolChatError(w, req.Model, stream, requestWantsSSE(r), fmt.Errorf("decoding response: %w", err))
		return
	}
	firstResp = normalizeOpenAIToolResponse(firstResp, searchReq.Tools)

	searchQuery := extractWebSearchQuery(firstResp)
	if searchQuery == "" {
		s.streamFirstResponse(w, r, req, firstResp, stream)
		return
	}

	log.Printf("WEBSEARCH: query=%q", searchQuery)

	sse := requestWantsSSE(r)
	if stream && sse {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		writeSSE(w, map[string]interface{}{"searching": searchQuery})
	}

	results, err := websearch.Search(r.Context(), searchQuery, 5)
	if err != nil {
		log.Printf("WEBSEARCH: search failed: %v", err)
		results = nil
	}

	log.Printf("WEBSEARCH: got %d results", len(results))

	toolResultContent := formatSearchResults(searchQuery, results)

	assistantMsg := buildAssistantToolCallMessage(firstResp)
	toolResultMsg := api.Message{
		Role:       "tool",
		Content:    toolResultContent,
		ToolName:   webSearchToolName,
		ToolCallID: extractToolCallID(firstResp),
	}

	followupReq := searchReq
	followupReq.Messages = append(followupReq.Messages, assistantMsg, toolResultMsg)

	followupOpenAI, err := ollamaChatRequestToOpenAI(followupReq, opts)
	if err != nil {
		s.writeToolChatError(w, req.Model, stream, sse, fmt.Errorf("building followup: %w", err))
		return
	}


	if stream {
		followupOpenAI["stream"] = true
		s.streamFollowupResponse(w, r.Context(), req.Model, proxy, followupOpenAI, sse)
	} else {
		followupOpenAI["stream"] = false
		s.nonStreamFollowupResponse(w, r.Context(), req.Model, proxy, followupOpenAI)
	}
}

func extractWebSearchQuery(resp api.OpenAIChatResponse) string {
	if len(resp.Choices) == 0 || resp.Choices[0].Message == nil {
		return ""
	}
	msg := resp.Choices[0].Message
	for _, tc := range msg.ToolCalls {
		if tc.Function.Name == webSearchToolName {
			args := toolArgumentsJSONString(tc.Function.Arguments)
			var parsed struct {
				Query string `json:"query"`
			}
			if json.Unmarshal([]byte(args), &parsed) == nil && parsed.Query != "" {
				return parsed.Query
			}
		}
	}
	return ""
}

func formatSearchResults(query string, results []websearch.Result) string {
	if len(results) == 0 {
		return fmt.Sprintf("Web search for %q returned no results.", query)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Web search results for %q:\n\n", query)
	for i, r := range results {
		fmt.Fprintf(&b, "%d. %s\n   URL: %s\n", i+1, r.Title, r.URL)
		if r.Snippet != "" {
			fmt.Fprintf(&b, "   %s\n", r.Snippet)
		}
		b.WriteString("\n")
	}
	return b.String()
}

func buildAssistantToolCallMessage(resp api.OpenAIChatResponse) api.Message {
	msg := resp.Choices[0].Message
	return api.Message{
		Role:      "assistant",
		Content:   contentOrEmpty(msg.Content),
		ToolCalls: msg.ToolCalls,
	}
}

func extractToolCallID(resp api.OpenAIChatResponse) string {
	if len(resp.Choices) == 0 || resp.Choices[0].Message == nil {
		return ""
	}
	for _, tc := range resp.Choices[0].Message.ToolCalls {
		if tc.Function.Name == webSearchToolName {
			return tc.ID
		}
	}
	return ""
}

func hasSystemMessage(msgs []api.Message) bool {
	for _, m := range msgs {
		if m.Role == "system" {
			return true
		}
	}
	return false
}

func appendToSystemMessage(msgs []api.Message, suffix string) []api.Message {
	out := make([]api.Message, len(msgs))
	copy(out, msgs)
	for i, m := range out {
		if m.Role == "system" {
			if s, ok := m.Content.(string); ok {
				out[i].Content = s + suffix
			}
			break
		}
	}
	return out
}

func (s *Server) streamFirstResponse(w http.ResponseWriter, r *http.Request, req api.ChatRequest, resp api.OpenAIChatResponse, stream bool) {
	ollamaResp, err := openAIChatResponseToOllama(req.Model, resp)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
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

		content := ""
		if ollamaResp.Message != nil {
			if s, ok := ollamaResp.Message.Content.(string); ok {
				content = s
			}
		}
		if content != "" {
			writeSSE(w, api.ChatResponse{
				Model:     req.Model,
				Message:   &api.Message{Role: "assistant", Content: content},
				Done:      false,
				CreatedAt: time.Now(),
			})
		}
		writeSSE(w, api.ChatResponse{Model: req.Model, Done: true, CreatedAt: time.Now()})
	} else {
		w.Header().Set("Content-Type", "application/x-ndjson")
		content := ""
		if ollamaResp.Message != nil {
			if s, ok := ollamaResp.Message.Content.(string); ok {
				content = s
			}
		}
		if content != "" {
			writeNDJSON(w, api.ChatResponse{
				Model:     req.Model,
				Message:   &api.Message{Role: "assistant", Content: content},
				Done:      false,
				CreatedAt: time.Now(),
			})
		}
		writeNDJSON(w, api.ChatResponse{
			Model:     req.Model,
			Message:   &api.Message{Role: "assistant", Content: ""},
			Done:      true,
			CreatedAt: time.Now(),
		})
	}
}

func (s *Server) streamFollowupResponse(w http.ResponseWriter, ctx context.Context, model string, proxy inference.ChatCompletionProxier, reqBody map[string]interface{}, sse bool) {
	resp, err := proxy.ChatCompletion(ctx, reqBody)
	if err != nil {
		if sse {
			writeSSE(w, api.ChatResponse{
				Model:     model,
				Message:   &api.Message{Role: "assistant", Content: "Error: " + err.Error()},
				Done:      true,
				CreatedAt: time.Now(),
			})
		}
		return
	}
	defer resp.Body.Close()

	if sse {
		s.relayOpenAIStreamAsSSE(w, resp, model)
	} else {
		w.Header().Set("Content-Type", "application/x-ndjson")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		s.relayOpenAIStreamAsNDJSON(w, resp, model)
	}
}

func (s *Server) nonStreamFollowupResponse(w http.ResponseWriter, ctx context.Context, model string, proxy inference.ChatCompletionProxier, reqBody map[string]interface{}) {
	resp, err := proxy.ChatCompletion(ctx, reqBody)
	if err != nil {
		writeInferenceError(w, err)
		return
	}
	defer resp.Body.Close()

	var openAIResp api.OpenAIChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&openAIResp); err != nil {
		writeError(w, http.StatusInternalServerError, "decoding followup response: "+err.Error())
		return
	}

	ollamaResp, err := openAIChatResponseToOllama(model, openAIResp)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	ollamaResp.Done = true
	writeJSON(w, http.StatusOK, ollamaResp)
}

func (s *Server) relayOpenAIStreamAsSSE(w http.ResponseWriter, resp *http.Response, model string) {
	buf := make([]byte, 4096)
	var partial string
	wroteChunk := false

	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			partial += string(buf[:n])
			for {
				newline := strings.Index(partial, "\n")
				if newline < 0 {
					break
				}
				line := strings.TrimSpace(partial[:newline])
				partial = partial[newline+1:]

				if !strings.HasPrefix(line, "data: ") {
					continue
				}
				data := line[6:]
				if data == "[DONE]" {
					writeSSE(w, api.ChatResponse{Model: model, Done: true, CreatedAt: time.Now()})
					return
				}

				var chunk struct {
					Choices []struct {
						Delta struct {
							Content string `json:"content"`
						} `json:"delta"`
					} `json:"choices"`
				}
				if json.Unmarshal([]byte(data), &chunk) == nil && len(chunk.Choices) > 0 {
					token := chunk.Choices[0].Delta.Content
					if token != "" {
						wroteChunk = true
						writeSSE(w, api.ChatResponse{
							Model:     model,
							Message:   &api.Message{Role: "assistant", Content: token},
							Done:      false,
							CreatedAt: time.Now(),
						})
					}
				}
			}
		}
		if err != nil {
			break
		}
	}

	_ = wroteChunk
	writeSSE(w, api.ChatResponse{Model: model, Done: true, CreatedAt: time.Now()})
}

func (s *Server) relayOpenAIStreamAsNDJSON(w http.ResponseWriter, resp *http.Response, model string) {
	buf := make([]byte, 4096)
	var partial string

	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			partial += string(buf[:n])
			for {
				newline := strings.Index(partial, "\n")
				if newline < 0 {
					break
				}
				line := strings.TrimSpace(partial[:newline])
				partial = partial[newline+1:]

				if !strings.HasPrefix(line, "data: ") {
					continue
				}
				data := line[6:]
				if data == "[DONE]" {
					writeNDJSON(w, api.ChatResponse{
						Model:     model,
						Message:   &api.Message{Role: "assistant", Content: ""},
						Done:      true,
						CreatedAt: time.Now(),
					})
					return
				}

				var chunk struct {
					Choices []struct {
						Delta struct {
							Content string `json:"content"`
						} `json:"delta"`
					} `json:"choices"`
				}
				if json.Unmarshal([]byte(data), &chunk) == nil && len(chunk.Choices) > 0 {
					token := chunk.Choices[0].Delta.Content
					if token != "" {
						writeNDJSON(w, api.ChatResponse{
							Model:     model,
							Message:   &api.Message{Role: "assistant", Content: token},
							Done:      false,
							CreatedAt: time.Now(),
						})
					}
				}
			}
		}
		if err != nil {
			break
		}
	}

	writeNDJSON(w, api.ChatResponse{
		Model:     model,
		Message:   &api.Message{Role: "assistant", Content: ""},
		Done:      true,
		CreatedAt: time.Now(),
	})
}
