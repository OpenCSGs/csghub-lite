package cloud

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/opencsgs/csghub-lite/pkg/api"
)

const (
	DefaultBaseURL        = "https://ai.space.opencsg.com"
	DefaultLoginURL       = "https://iam.opencsg.com/login/oauth/authorize?client_id=d623c957e69976c8a7a8&response_type=code&redirect_uri=https://hub.opencsg.com/api/v1/callback/casdoor&scope=read&state=casdoor"
	DefaultAccessTokenURL = "https://opencsg.com/settings/access-token"
	defaultCacheTTL       = 5 * time.Minute
)

type Service struct {
	baseURL string
	client  *http.Client
	ttl     time.Duration

	mu       sync.RWMutex
	cached   []api.ModelInfo
	cachedAt time.Time
}

type modelListResponse struct {
	Object string        `json:"object"`
	Data   []remoteModel `json:"data"`
}

type remoteModel struct {
	ID          string                 `json:"id"`
	Object      string                 `json:"object"`
	Created     int64                  `json:"created"`
	OwnedBy     string                 `json:"owned_by"`
	Task        string                 `json:"task"`
	DisplayName string                 `json:"display_name"`
	Public      bool                   `json:"public"`
	Metadata    map[string]interface{} `json:"metadata"`
}

func NewService(baseURL string) *Service {
	return &Service{
		baseURL: strings.TrimRight(baseURL, "/"),
		client:  &http.Client{Timeout: 15 * time.Second},
		ttl:     defaultCacheTTL,
	}
}

func (s *Service) BaseURL() string {
	if s == nil {
		return ""
	}
	return s.baseURL
}

func (s *Service) ListChatModels(ctx context.Context) ([]api.ModelInfo, error) {
	if s == nil || s.baseURL == "" {
		return nil, nil
	}
	if models, ok := s.cachedModels(); ok {
		return models, nil
	}
	return s.refresh(ctx)
}

func (s *Service) RefreshChatModels(ctx context.Context) ([]api.ModelInfo, error) {
	if s == nil || s.baseURL == "" {
		return nil, nil
	}
	return s.refresh(ctx)
}

func (s *Service) InvalidateChatModels() {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.cached = nil
	s.cachedAt = time.Time{}
	s.mu.Unlock()
}

func (s *Service) cachedModels() ([]api.ModelInfo, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.cached) == 0 || time.Since(s.cachedAt) > s.ttl {
		return nil, false
	}
	return cloneModels(s.cached), true
}

func (s *Service) refresh(ctx context.Context) ([]api.ModelInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.baseURL+"/v1/models", nil)
	if err != nil {
		return nil, fmt.Errorf("creating cloud model request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching cloud models: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("cloud model list returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var payload modelListResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decoding cloud model list: %w", err)
	}

	models := make([]api.ModelInfo, 0, len(payload.Data))
	for _, item := range payload.Data {
		info, ok := modelInfoFromRemote(item)
		if !ok {
			continue
		}
		models = append(models, info)
	}

	s.mu.Lock()
	s.cached = cloneModels(models)
	s.cachedAt = time.Now()
	s.mu.Unlock()

	return models, nil
}

func modelInfoFromRemote(item remoteModel) (api.ModelInfo, bool) {
	if !supportsChat(item) {
		return api.ModelInfo{}, false
	}

	displayName := strings.TrimSpace(item.DisplayName)
	if displayName == "" {
		displayName = strings.TrimSpace(item.ID)
	}

	pipelineTag := strings.TrimSpace(item.Task)
	if pipelineTag == "" {
		pipelineTag = "text-generation"
	}

	var modifiedAt time.Time
	if item.Created > 0 {
		modifiedAt = time.Unix(item.Created, 0).UTC()
	}

	return api.ModelInfo{
		Name:        item.ID,
		Model:       item.ID,
		Format:      "cloud",
		ModifiedAt:  modifiedAt,
		DisplayName: displayName,
		Source:      "cloud",
		PipelineTag: pipelineTag,
		HasMMProj:   item.Task == "image-text-to-text",
	}, true
}

func supportsChat(item remoteModel) bool {
	task := strings.TrimSpace(strings.ToLower(item.Task))
	switch task {
	case "text-generation", "image-text-to-text":
		return true
	case "":
		return true
	default:
		return false
	}
}

func cloneModels(models []api.ModelInfo) []api.ModelInfo {
	if len(models) == 0 {
		return nil
	}
	out := make([]api.ModelInfo, len(models))
	copy(out, models)
	return out
}
