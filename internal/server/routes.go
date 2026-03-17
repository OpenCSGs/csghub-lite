package server

import "net/http"

func (s *Server) routes() *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /", s.handleHealth)
	mux.HandleFunc("GET /api/health", s.handleHealth)
	mux.HandleFunc("GET /api/tags", s.handleTags)
	mux.HandleFunc("GET /api/ps", s.handlePs)
	mux.HandleFunc("POST /api/show", s.handleShow)
	mux.HandleFunc("POST /api/pull", s.handlePull)
	mux.HandleFunc("POST /api/stop", s.handleStop)
	mux.HandleFunc("DELETE /api/delete", s.handleDelete)
	mux.HandleFunc("POST /api/generate", s.handleGenerate)
	mux.HandleFunc("POST /api/chat", s.handleChat)

	mux.HandleFunc("GET /api/datasets", s.handleDatasetTags)
	mux.HandleFunc("POST /api/datasets/show", s.handleDatasetShow)
	mux.HandleFunc("POST /api/datasets/pull", s.handleDatasetPull)
	mux.HandleFunc("DELETE /api/datasets/delete", s.handleDatasetDelete)

	mux.HandleFunc("POST /v1/chat/completions", s.handleOpenAIChatCompletions)
	mux.HandleFunc("GET /v1/models", s.handleOpenAIModels)

	return mux
}
