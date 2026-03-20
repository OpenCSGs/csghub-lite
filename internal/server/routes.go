package server

import "net/http"

func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()

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

	// New: marketplace, system, logs, settings
	mux.HandleFunc("GET /api/marketplace/models", s.handleMarketplaceModels)
	mux.HandleFunc("GET /api/marketplace/datasets", s.handleMarketplaceDatasets)
	mux.HandleFunc("GET /api/system", s.handleSystem)
	mux.HandleFunc("GET /api/settings", s.handleSettings)
	mux.HandleFunc("GET /api/logs", s.handleLogs)

	// Static files: serve embedded web UI or dev fallback
	if hasEmbeddedStatic() {
		mux.Handle("GET /", staticHandler())
	} else {
		mux.Handle("GET /", devStaticHandler("web/dist"))
	}

	return corsMiddleware(LogMiddleware(mux))
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
