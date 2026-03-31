package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/opencsgs/csghub-lite/internal/csghub"
	"github.com/opencsgs/csghub-lite/internal/inference"
	"github.com/opencsgs/csghub-lite/pkg/api"
)

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func writeInferenceError(w http.ResponseWriter, err error) {
	status := inference.HTTPStatusCode(err)
	if status == 0 {
		status = http.StatusInternalServerError
	}
	writeError(w, status, inference.HTTPErrorMessage(err))
}

func writeSSE(w http.ResponseWriter, v interface{}) {
	data, _ := json.Marshal(v)
	fmt.Fprintf(w, "data: %s\n\n", data)
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}

func writeNDJSON(w http.ResponseWriter, v interface{}) {
	data, _ := json.Marshal(v)
	fmt.Fprintf(w, "%s\n", data)
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}

func requestWantsSSE(r *http.Request) bool {
	if strings.EqualFold(r.Header.Get("X-CSGHUB-Stream"), "sse") {
		return true
	}
	return strings.Contains(strings.ToLower(r.Header.Get("Accept")), "text/event-stream")
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// GET /api/tags -- list local models
func (s *Server) handleTags(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-cache")

	infos, err := s.listLocalModelInfos()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	cloudModels, err := s.listCloudModels(r.Context(), requestWantsModelRefresh(r))
	if err != nil {
		log.Printf("cloud models unavailable: %v", err)
	} else {
		infos = append(infos, cloudModels...)
	}

	writeJSON(w, http.StatusOK, api.TagsResponse{Models: infos})
}

// GET /api/ps -- list running models
func (s *Server) handlePs(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var models []api.RunningModel
	for id, me := range s.engines {
		lm, err := s.manager.Get(id)
		if err != nil {
			continue
		}
		models = append(models, api.RunningModel{
			Name:      lm.FullName(),
			Model:     lm.FullName(),
			Size:      lm.Size,
			Format:    string(lm.Format),
			ExpiresAt: me.expiresAt(),
		})
	}

	writeJSON(w, http.StatusOK, api.PsResponse{Models: models})
}

// POST /api/stop -- unload a model
func (s *Server) handleStop(w http.ResponseWriter, r *http.Request) {
	var req api.StopRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	s.mu.Lock()
	me, ok := s.engines[req.Model]
	if ok {
		me.engine.Close()
		delete(s.engines, req.Model)
	}
	s.mu.Unlock()

	if !ok {
		writeError(w, http.StatusNotFound, fmt.Sprintf("model %q is not running", req.Model))
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "stopped"})
}

// POST /api/show -- model details
func (s *Server) handleShow(w http.ResponseWriter, r *http.Request) {
	var req api.ShowRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	lm, err := s.manager.Get(req.Model)
	if err != nil {
		writeError(w, http.StatusNotFound, fmt.Sprintf("model %q not found", req.Model))
		return
	}

	writeJSON(w, http.StatusOK, api.ShowResponse{
		Details: s.localModelInfo(lm),
	})
}

// POST /api/pull -- download a model
func (s *Server) handlePull(w http.ResponseWriter, r *http.Request) {
	var req api.PullRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	var mu sync.Mutex
	safeSSE := func(v interface{}) {
		mu.Lock()
		writeSSE(w, v)
		mu.Unlock()
	}

	safeSSE(api.PullResponse{Status: "pulling " + req.Model})

	progress := func(p csghub.SnapshotProgress) {
		safeSSE(api.PullResponse{
			Status:    fmt.Sprintf("downloading %s", p.FileName),
			Digest:    p.FileName,
			Total:     p.BytesTotal,
			Completed: p.BytesCompleted,
		})
	}

	_, err := s.manager.Pull(r.Context(), req.Model, progress)
	if err != nil {
		log.Printf("pull %s failed: %v", req.Model, err)
		safeSSE(api.PullResponse{Status: "error: " + err.Error()})
		return
	}

	safeSSE(api.PullResponse{Status: "success"})
}

// DELETE /api/delete -- remove a model
func (s *Server) handleDelete(w http.ResponseWriter, r *http.Request) {
	var req api.DeleteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Close engine if running
	s.mu.Lock()
	if me, ok := s.engines[req.Model]; ok {
		me.engine.Close()
		delete(s.engines, req.Model)
	}
	s.mu.Unlock()

	if err := s.manager.Remove(req.Model); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// POST /api/load -- eagerly load (and convert if necessary) a model
func (s *Server) handleLoad(w http.ResponseWriter, r *http.Request) {
	var req api.LoadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	stream := req.Stream != nil && *req.Stream

	if !stream {
		_, err := s.getOrLoadEngine(req.Model)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		s.touchEngine(req.Model)
		writeJSON(w, http.StatusOK, api.LoadResponse{Status: "ready"})
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	var mu sync.Mutex
	safeSSE := func(v interface{}) {
		mu.Lock()
		writeSSE(w, v)
		mu.Unlock()
	}

	safeSSE(api.LoadResponse{Status: "loading " + req.Model})

	progress := func(step string, current, total int) {
		safeSSE(api.LoadResponse{
			Status:  "converting",
			Step:    step,
			Current: current,
			Total:   total,
		})
	}

	_, err := s.getOrLoadEngineWithProgress(req.Model, progress)
	if err != nil {
		log.Printf("load %s failed: %v", req.Model, err)
		safeSSE(api.LoadResponse{Status: "error: " + err.Error()})
		return
	}
	s.touchEngine(req.Model)

	safeSSE(api.LoadResponse{Status: "ready"})
}

// POST /api/generate -- text generation
func (s *Server) handleGenerate(w http.ResponseWriter, r *http.Request) {
	var req api.GenerateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	opts := inference.DefaultOptions()
	requestedNumCtx := 0
	if req.Options != nil {
		if req.Options.Temperature > 0 {
			opts.Temperature = req.Options.Temperature
		}
		if req.Options.TopP > 0 {
			opts.TopP = req.Options.TopP
		}
		if req.Options.TopK > 0 {
			opts.TopK = req.Options.TopK
		}
		if req.Options.MaxTokens > 0 {
			opts.MaxTokens = req.Options.MaxTokens
		}
		if req.Options.NumCtx > 0 {
			opts.NumCtx = req.Options.NumCtx
			requestedNumCtx = req.Options.NumCtx
		}
	}

	eng, err := s.getOrLoadEngineWithNumCtx(req.Model, requestedNumCtx)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	defer s.touchEngine(req.Model)

	stream := req.Stream == nil || *req.Stream

	if stream {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		onToken := func(token string) {
			writeSSE(w, api.GenerateResponse{
				Model:     req.Model,
				Response:  token,
				Done:      false,
				CreatedAt: time.Now(),
			})
		}

		_, err := eng.Generate(r.Context(), req.Prompt, opts, onToken)
		if err != nil {
			writeSSE(w, api.GenerateResponse{
				Model:     req.Model,
				Response:  "Error: " + err.Error(),
				Done:      true,
				CreatedAt: time.Now(),
			})
			return
		}
		writeSSE(w, api.GenerateResponse{
			Model:     req.Model,
			Done:      true,
			CreatedAt: time.Now(),
		})
	} else {
		response, err := eng.Generate(r.Context(), req.Prompt, opts, nil)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, api.GenerateResponse{
			Model:     req.Model,
			Response:  response,
			Done:      true,
			CreatedAt: time.Now(),
		})
	}
}

// POST /api/chat -- chat completions
func (s *Server) handleChat(w http.ResponseWriter, r *http.Request) {
	var req api.ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	opts := inference.DefaultOptions()
	requestedNumCtx := 0
	if req.Options != nil {
		if req.Options.Temperature > 0 {
			opts.Temperature = req.Options.Temperature
		}
		if req.Options.TopP > 0 {
			opts.TopP = req.Options.TopP
		}
		if req.Options.TopK > 0 {
			opts.TopK = req.Options.TopK
		}
		if req.Options.MaxTokens > 0 {
			opts.MaxTokens = req.Options.MaxTokens
		}
		if req.Options.NumCtx > 0 {
			opts.NumCtx = req.Options.NumCtx
			requestedNumCtx = req.Options.NumCtx
		}
	}

	eng, err := s.getChatEngine(r.Context(), req.Model, req.Source, requestedNumCtx)
	if err != nil {
		writeInferenceError(w, err)
		return
	}
	defer s.touchEngine(req.Model)

	var messages []inference.Message
	for _, m := range req.Messages {
		messages = append(messages, inference.Message{Role: m.Role, Content: m.Content})
	}

	stream := req.Stream == nil || *req.Stream
	if hasToolChatFeatures(req) {
		s.handleChatWithTools(w, r, req, eng, opts, stream)
		return
	}

	if stream {
		if requestWantsSSE(r) {
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Connection", "keep-alive")

			wroteChunk := false
			onToken := func(token string) {
				wroteChunk = true
				writeSSE(w, api.ChatResponse{
					Model: req.Model,
					Message: &api.Message{
						Role:    "assistant",
						Content: token,
					},
					Done:      false,
					CreatedAt: time.Now(),
				})
			}

			fullResp, err := eng.Chat(r.Context(), messages, opts, onToken)
			if err != nil {
				if !wroteChunk {
					writeInferenceError(w, err)
					return
				}
				writeSSE(w, api.ChatResponse{
					Model: req.Model,
					Message: &api.Message{
						Role:    "assistant",
						Content: "Error: " + err.Error(),
					},
					Done:      true,
					CreatedAt: time.Now(),
				})
				return
			}
			_ = fullResp
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

		wroteChunk := false
		onToken := func(token string) {
			wroteChunk = true
			writeNDJSON(w, api.ChatResponse{
				Model: req.Model,
				Message: &api.Message{
					Role:    "assistant",
					Content: token,
				},
				Done:      false,
				CreatedAt: time.Now(),
			})
		}

		_, err := eng.Chat(r.Context(), messages, opts, onToken)
		if err != nil {
			if !wroteChunk {
				writeInferenceError(w, err)
				return
			}
			writeNDJSON(w, api.ChatResponse{
				Model: req.Model,
				Message: &api.Message{
					Role:    "assistant",
					Content: "Error: " + err.Error(),
				},
				Done:      true,
				CreatedAt: time.Now(),
			})
			return
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
	} else {
		response, err := eng.Chat(r.Context(), messages, opts, nil)
		if err != nil {
			writeInferenceError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, api.ChatResponse{
			Model: req.Model,
			Message: &api.Message{
				Role:    "assistant",
				Content: response,
			},
			Done:      true,
			CreatedAt: time.Now(),
		})
	}
}
