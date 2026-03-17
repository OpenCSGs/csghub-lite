package server

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/opencsgs/csghub-lite/internal/config"
	"github.com/opencsgs/csghub-lite/internal/dataset"
	"github.com/opencsgs/csghub-lite/internal/inference"
	"github.com/opencsgs/csghub-lite/internal/model"
)

const (
	DefaultKeepAlive  = 5 * time.Minute
	evictorInterval   = 30 * time.Second
)

type managedEngine struct {
	engine    inference.Engine
	lastUsed  time.Time
	keepAlive time.Duration
}

func (m *managedEngine) expiresAt() time.Time {
	return m.lastUsed.Add(m.keepAlive)
}

type Server struct {
	cfg            *config.Config
	manager        *model.Manager
	datasetManager *dataset.Manager
	http           *http.Server

	mu      sync.RWMutex
	engines map[string]*managedEngine
}

func New(cfg *config.Config) *Server {
	mgr := model.NewManager(cfg)
	dsMgr := dataset.NewManager(cfg)
	s := &Server{
		cfg:            cfg,
		manager:        mgr,
		datasetManager: dsMgr,
		engines:        make(map[string]*managedEngine),
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

	go s.startEvictor(ctx)

	errCh := make(chan error, 1)
	go func() {
		addr := s.cfg.ListenAddr
		if strings.HasPrefix(addr, ":") {
			addr = "localhost" + addr
		}
		fmt.Printf("csghub-lite server listening on %s\n", s.cfg.ListenAddr)
		fmt.Printf("  Ollama API: http://%s/api/chat\n", addr)
		fmt.Printf("  OpenAI API: http://%s/v1/chat/completions\n", addr)
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

// startEvictor periodically closes engines that have exceeded their keep-alive.
func (s *Server) startEvictor(ctx context.Context) {
	ticker := time.NewTicker(evictorInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			s.evictExpired(now)
		}
	}
}

func (s *Server) evictExpired(now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, me := range s.engines {
		if now.After(me.expiresAt()) {
			log.Printf("evicting idle model %s (unused for %s)", id, me.keepAlive)
			me.engine.Close()
			delete(s.engines, id)
		}
	}
}

// touchEngine updates lastUsed for the given model. Must be called after
// every inference request so the evictor knows the engine is still active.
func (s *Server) touchEngine(modelID string) {
	s.mu.Lock()
	if me, ok := s.engines[modelID]; ok {
		me.lastUsed = time.Now()
	}
	s.mu.Unlock()
}

func (s *Server) getOrLoadEngine(modelID string) (inference.Engine, error) {
	s.mu.RLock()
	me, ok := s.engines[modelID]
	s.mu.RUnlock()
	if ok {
		return me.engine, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if me, ok := s.engines[modelID]; ok {
		return me.engine, nil
	}

	modelDir, err := s.manager.ModelPath(modelID)
	if err != nil {
		return nil, fmt.Errorf("model %q not found locally; use 'csghub-lite pull %s' first", modelID, modelID)
	}

	lm, err := s.manager.Get(modelID)
	if err != nil {
		return nil, err
	}

	eng, err := inference.LoadEngine(modelDir, lm)
	if err != nil {
		return nil, err
	}

	s.engines[modelID] = &managedEngine{
		engine:    eng,
		lastUsed:  time.Now(),
		keepAlive: DefaultKeepAlive,
	}
	return eng, nil
}

func (s *Server) closeAllEngines() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, me := range s.engines {
		me.engine.Close()
		delete(s.engines, id)
	}
}
