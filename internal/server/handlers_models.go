package server

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/opencsgs/csghub-lite/pkg/api"
)

const (
	defaultAnthropicMaxInputTokens = 32768
	defaultAnthropicMaxTokens      = 8192
)

func (s *Server) handleModels(w http.ResponseWriter, r *http.Request) {
	if requestWantsAnthropicModels(r) {
		s.handleAnthropicModels(w, r)
		return
	}
	s.handleOpenAIModels(w, r)
}

func requestWantsAnthropicModels(r *http.Request) bool {
	if strings.TrimSpace(r.Header.Get("anthropic-version")) != "" {
		return true
	}
	if strings.TrimSpace(r.Header.Get("anthropic-beta")) != "" {
		return true
	}
	if strings.TrimSpace(r.Header.Get("x-api-key")) != "" && strings.TrimSpace(r.Header.Get("authorization")) == "" {
		return true
	}
	return strings.Contains(strings.ToLower(strings.TrimSpace(r.UserAgent())), "claude")
}

func (s *Server) listAvailableModels(ctx context.Context) ([]api.ModelInfo, error) {
	localModels, err := s.manager.List()
	if err != nil {
		return nil, err
	}

	seen := make(map[string]struct{}, len(localModels)+8)
	out := make([]api.ModelInfo, 0, len(localModels)+8)
	for _, item := range localModels {
		modelID := strings.TrimSpace(item.FullName())
		if modelID == "" {
			continue
		}
		if _, ok := seen[modelID]; ok {
			continue
		}
		seen[modelID] = struct{}{}
		out = append(out, api.ModelInfo{
			Name:        modelID,
			Model:       modelID,
			ModifiedAt:  item.DownloadedAt,
			DisplayName: modelID,
			Source:      "local",
			PipelineTag: "text-generation",
		})
	}

	if s.cloud != nil && strings.TrimSpace(s.cfg.Token) != "" {
		if cloudModels, err := s.cloud.ListChatModels(ctx); err == nil {
			for _, item := range cloudModels {
				modelID := strings.TrimSpace(item.Model)
				if modelID == "" {
					continue
				}
				if _, ok := seen[modelID]; ok {
					continue
				}
				seen[modelID] = struct{}{}
				out = append(out, item)
			}
		}
	}

	return out, nil
}

func (s *Server) handleAnthropicModels(w http.ResponseWriter, r *http.Request) {
	models, err := s.listAvailableModels(r.Context())
	if err != nil {
		writeAnthropicError(w, http.StatusInternalServerError, err.Error())
		return
	}

	data := make([]api.AnthropicModelInfo, 0, len(models))
	for _, item := range models {
		data = append(data, anthropicModelFromInfo(item))
	}

	resp := api.AnthropicModelListResponse{
		Data:    data,
		HasMore: false,
	}
	if len(data) > 0 {
		resp.FirstID = data[0].ID
		resp.LastID = data[len(data)-1].ID
	}
	writeJSON(w, http.StatusOK, resp)
}

func anthropicModelFromInfo(item api.ModelInfo) api.AnthropicModelInfo {
	createdAt := time.Unix(0, 0).UTC().Format(time.RFC3339)
	if !item.ModifiedAt.IsZero() {
		createdAt = item.ModifiedAt.UTC().Format(time.RFC3339)
	}

	displayName := strings.TrimSpace(item.DisplayName)
	if displayName == "" {
		displayName = strings.TrimSpace(item.Model)
	}

	supportsVision := item.HasMMProj || strings.EqualFold(strings.TrimSpace(item.PipelineTag), "image-text-to-text")
	supportsThinking := strings.Contains(strings.ToLower(item.Model), "thinking")

	return api.AnthropicModelInfo{
		ID:             strings.TrimSpace(item.Model),
		Type:           "model",
		DisplayName:    displayName,
		CreatedAt:      createdAt,
		MaxInputTokens: defaultAnthropicMaxInputTokens,
		MaxTokens:      defaultAnthropicMaxTokens,
		Capabilities: api.AnthropicModelCapabilities{
			Batch:             api.AnthropicCapabilitySupport{Supported: false},
			Citations:         api.AnthropicCapabilitySupport{Supported: false},
			CodeExecution:     api.AnthropicCapabilitySupport{Supported: false},
			ContextManagement: api.AnthropicContextManagementCapability{Supported: false},
			ImageInput:        api.AnthropicCapabilitySupport{Supported: supportsVision},
			PDFInput:          api.AnthropicCapabilitySupport{Supported: false},
			StructuredOutputs: api.AnthropicCapabilitySupport{Supported: false},
			Thinking: api.AnthropicThinkingCapability{
				Supported: supportsThinking,
				Types: api.AnthropicThinkingTypes{
					Adaptive: api.AnthropicCapabilitySupport{Supported: supportsThinking},
					Enabled:  api.AnthropicCapabilitySupport{Supported: supportsThinking},
				},
			},
		},
	}
}
