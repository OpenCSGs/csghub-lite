package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/opencsgs/csghub-lite/internal/inference"
	"github.com/opencsgs/csghub-lite/pkg/api"
)

// POST /v1/chat/completions -- OpenAI-compatible chat completions
func (s *Server) handleOpenAIChatCompletions(w http.ResponseWriter, r *http.Request) {
	var req api.OpenAIChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeOpenAIError(w, http.StatusBadRequest, "invalid_request_error", "invalid request body")
		return
	}
	if req.Model == "" {
		writeOpenAIError(w, http.StatusBadRequest, "invalid_request_error", "model is required")
		return
	}

	opts := inference.DefaultOptions()
	requestedNumCtx := 0
	if req.Temperature != nil {
		opts.Temperature = *req.Temperature
	}
	if req.TopP != nil {
		opts.TopP = *req.TopP
	}
	if req.MaxTokens != nil {
		opts.MaxTokens = *req.MaxTokens
	}
	if req.NumCtx != nil && *req.NumCtx > 0 {
		opts.NumCtx = *req.NumCtx
		requestedNumCtx = *req.NumCtx
	}
	if req.Seed != nil {
		opts.Seed = *req.Seed
	}
	if len(req.Stop) > 0 {
		opts.Stop = req.Stop
	}

	eng, err := s.getOrLoadEngineWithNumCtx(req.Model, requestedNumCtx)
	if err != nil {
		writeOpenAIError(w, http.StatusBadRequest, "model_not_found", err.Error())
		return
	}
	defer s.touchEngine(req.Model)

	var messages []inference.Message
	for _, m := range req.Messages {
		messages = append(messages, inference.Message{Role: m.Role, Content: m.Content})
	}

	id := fmt.Sprintf("chatcmpl-%d", time.Now().UnixNano())
	created := time.Now().Unix()
	stream := req.Stream != nil && *req.Stream

	if stream {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		onToken := func(token string) {
			chunk := api.OpenAIChatResponse{
				ID:      id,
				Object:  "chat.completion.chunk",
				Created: created,
				Model:   req.Model,
				Choices: []api.OpenAIChoice{{
					Index: 0,
					Delta: &api.Message{Role: "assistant", Content: token},
				}},
			}
			writeSSE(w, chunk)
		}

		_, err := eng.Chat(r.Context(), messages, opts, onToken)
		if err != nil {
			writeSSE(w, map[string]string{"error": err.Error()})
			return
		}

		stop := "stop"
		writeSSE(w, api.OpenAIChatResponse{
			ID:      id,
			Object:  "chat.completion.chunk",
			Created: created,
			Model:   req.Model,
			Choices: []api.OpenAIChoice{{
				Index:        0,
				Delta:        &api.Message{Role: "assistant", Content: ""},
				FinishReason: &stop,
			}},
		})
		fmt.Fprintf(w, "data: [DONE]\n\n")
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	} else {
		response, err := eng.Chat(r.Context(), messages, opts, nil)
		if err != nil {
			writeOpenAIError(w, http.StatusInternalServerError, "server_error", err.Error())
			return
		}

		stop := "stop"
		writeJSON(w, http.StatusOK, api.OpenAIChatResponse{
			ID:      id,
			Object:  "chat.completion",
			Created: created,
			Model:   req.Model,
			Choices: []api.OpenAIChoice{{
				Index:        0,
				Message:      &api.Message{Role: "assistant", Content: response},
				FinishReason: &stop,
			}},
		})
	}
}

// GET /v1/models -- OpenAI-compatible model listing
func (s *Server) handleOpenAIModels(w http.ResponseWriter, r *http.Request) {
	models, err := s.manager.List()
	if err != nil {
		writeOpenAIError(w, http.StatusInternalServerError, "server_error", err.Error())
		return
	}

	var data []api.OpenAIModel
	for _, m := range models {
		data = append(data, api.OpenAIModel{
			ID:      m.FullName(),
			Object:  "model",
			Created: m.DownloadedAt.Unix(),
			OwnedBy: "csghub-lite",
		})
	}

	writeJSON(w, http.StatusOK, api.OpenAIModelList{
		Object: "list",
		Data:   data,
	})
}

func writeOpenAIError(w http.ResponseWriter, status int, errType, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]interface{}{
			"message": msg,
			"type":    errType,
		},
	})
}
