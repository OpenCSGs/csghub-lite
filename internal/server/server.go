package server

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/opencsgs/csghub-lite/internal/apps"
	"github.com/opencsgs/csghub-lite/internal/cloud"
	"github.com/opencsgs/csghub-lite/internal/config"
	"github.com/opencsgs/csghub-lite/internal/dataset"
	"github.com/opencsgs/csghub-lite/internal/inference"
	"github.com/opencsgs/csghub-lite/internal/model"
)

const (
	DefaultKeepAlive = 5 * time.Minute
	evictorInterval  = 30 * time.Second
)

type managedEngine struct {
	engine    inference.Engine
	numCtx    int
	lastUsed  time.Time
	keepAlive time.Duration
}

func (m *managedEngine) expiresAt() time.Time {
	return m.lastUsed.Add(m.keepAlive)
}

type Server struct {
	cfg            *config.Config
	version        string
	manager        *model.Manager
	datasetManager *dataset.Manager
	appManager     *apps.Manager
	appShells      *aiAppShellManager
	cloud          *cloud.Service
	http           *http.Server
	logBuf         *LogBuffer

	mu      sync.RWMutex
	engines map[string]*managedEngine
	prefsMu sync.Mutex

	cloudRefreshMu   sync.Mutex
	cloudRefreshAt   time.Time
	cloudRefreshWait chan struct{}
}

func New(cfg *config.Config, version string) *Server {
	mgr := model.NewManager(cfg)
	dsMgr := dataset.NewManager(cfg)
	logBuf := NewLogBuffer(500)
	SetupLogging(logBuf)

	s := &Server{
		cfg:            cfg,
		version:        version,
		manager:        mgr,
		datasetManager: dsMgr,
		appManager:     apps.NewManager(cfg),
		cloud:          cloud.NewService(cloud.DefaultBaseURL),
		engines:        make(map[string]*managedEngine),
		logBuf:         logBuf,
	}
	s.appShells = newAIAppShellManager()

	handler := s.routes()
	s.http = &http.Server{
		Addr:         cfg.ListenAddr,
		Handler:      handler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 0, // streaming responses
		IdleTimeout:  120 * time.Second,
	}
	return s
}

func (s *Server) Run(ctx context.Context) error {
	ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Port conflict detection
	if err := checkPort(s.cfg.ListenAddr); err != nil {
		return fmt.Errorf("port %s is already in use; try a different port with --listen :PORT\n  %w", s.cfg.ListenAddr, err)
	}

	go s.startEvictor(ctx)

	errCh := make(chan error, 1)
	go func() {
		addr := s.cfg.ListenAddr
		if strings.HasPrefix(addr, ":") {
			addr = "localhost" + addr
		}
		fmt.Printf("csghub-lite server listening on %s\n", s.cfg.ListenAddr)
		fmt.Printf("  Web UI:     http://%s/\n", addr)
		fmt.Printf("  Ollama API: http://%s/api/chat\n", addr)
		fmt.Printf("  OpenAI API: http://%s/v1/chat/completions\n", addr)
		fmt.Printf("  Anthropic API: http://%s/v1/messages\n", addr)
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

// checkPort attempts to listen on the address to detect conflicts early.
func checkPort(addr string) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	ln.Close()
	return nil
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
	return s.getOrLoadEngineWithProgressAndNumCtx(modelID, nil, 0)
}

func (s *Server) getOrLoadEngineWithProgress(modelID string, progress inference.ConvertProgressFunc) (inference.Engine, error) {
	return s.getOrLoadEngineWithProgressAndNumCtx(modelID, progress, 0)
}

func (s *Server) getOrLoadEngineWithNumCtx(modelID string, numCtx int) (inference.Engine, error) {
	return s.getOrLoadEngineWithProgressAndNumCtx(modelID, nil, numCtx)
}

func (s *Server) getOrLoadEngineWithProgressAndNumCtx(modelID string, progress inference.ConvertProgressFunc, numCtx int) (inference.Engine, error) {
	modelDir, err := s.manager.ModelPath(modelID)
	if err != nil {
		return nil, fmt.Errorf("model %q not found locally; use 'csghub-lite pull %s' first", modelID, modelID)
	}
	effectiveNumCtx := inference.ResolveNumCtx(modelDir, numCtx)

	s.mu.RLock()
	me, ok := s.engines[modelID]
	s.mu.RUnlock()
	if ok {
		if me.numCtx == effectiveNumCtx {
			return me.engine, nil
		}
		// fall through to replace engine with requested context window
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if me, ok := s.engines[modelID]; ok {
		if me.numCtx == effectiveNumCtx {
			return me.engine, nil
		}
		log.Printf("reloading model %s due to num_ctx change (%d -> %d)", modelID, me.numCtx, effectiveNumCtx)
		me.engine.Close()
		delete(s.engines, modelID)
	}

	lm, err := s.manager.Get(modelID)
	if err != nil {
		return nil, err
	}

	eng, err := inference.LoadEngineWithProgress(modelDir, lm, progress, false, effectiveNumCtx)
	if err != nil {
		return nil, err
	}

	s.engines[modelID] = &managedEngine{
		engine:    eng,
		numCtx:    effectiveNumCtx,
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
