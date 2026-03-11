package server

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/opencsgs/csghub-lite/internal/config"
	"github.com/opencsgs/csghub-lite/internal/inference"
	"github.com/opencsgs/csghub-lite/internal/model"
)

type Server struct {
	cfg     *config.Config
	manager *model.Manager
	http    *http.Server

	mu      sync.RWMutex
	engines map[string]inference.Engine
}

func New(cfg *config.Config) *Server {
	mgr := model.NewManager(cfg)
	s := &Server{
		cfg:     cfg,
		manager: mgr,
		engines: make(map[string]inference.Engine),
	}

	mux := s.routes()
	s.http = &http.Server{
		Addr:         cfg.ListenAddr,
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 0, // streaming responses
		IdleTimeout:  120 * time.Second,
	}
	return s
}

func (s *Server) Run(ctx context.Context) error {
	ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		fmt.Printf("csghub-lite server listening on %s\n", s.cfg.ListenAddr)
		if err := s.http.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		log.Println("shutting down server...")
		s.closeAllEngines()
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return s.http.Shutdown(shutCtx)
	}
}

func (s *Server) getOrLoadEngine(modelID string) (inference.Engine, error) {
	s.mu.RLock()
	eng, ok := s.engines[modelID]
	s.mu.RUnlock()
	if ok {
		return eng, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Double-check after acquiring write lock
	if eng, ok := s.engines[modelID]; ok {
		return eng, nil
	}

	modelDir, err := s.manager.ModelPath(modelID)
	if err != nil {
		return nil, fmt.Errorf("model %q not found locally; use 'csghub-lite pull %s' first", modelID, modelID)
	}

	lm, err := s.manager.Get(modelID)
	if err != nil {
		return nil, err
	}

	eng, err = inference.LoadEngine(modelDir, lm)
	if err != nil {
		return nil, err
	}

	s.engines[modelID] = eng
	return eng, nil
}

func (s *Server) closeAllEngines() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, eng := range s.engines {
		eng.Close()
		delete(s.engines, id)
	}
}
