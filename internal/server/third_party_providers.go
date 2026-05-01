package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/opencsgs/csghub-lite/internal/config"
	"github.com/opencsgs/csghub-lite/internal/inference"
	"github.com/opencsgs/csghub-lite/pkg/api"
)

const thirdPartyProviderSourcePrefix = "provider:"

func providerSource(id string) string {
	return thirdPartyProviderSourcePrefix + strings.TrimSpace(id)
}

func providerIDFromSource(source string) string {
	source = strings.TrimSpace(source)
	if !strings.HasPrefix(source, thirdPartyProviderSourcePrefix) {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(source, thirdPartyProviderSourcePrefix))
}

func getThirdPartyProvider(id string) (config.ThirdPartyProvider, bool) {
	id = strings.TrimSpace(id)
	if id == "" {
		return config.ThirdPartyProvider{}, false
	}
	for _, provider := range config.GetProviders() {
		if provider.ID == id {
			return provider, true
		}
	}
	return config.ThirdPartyProvider{}, false
}

func (s *Server) listThirdPartyProviderModels(ctx context.Context) []api.ModelInfo {
	var out []api.ModelInfo
	for _, provider := range config.GetProviders() {
		models, err := listOpenAICompatibleProviderModels(ctx, provider)
		if err != nil {
			continue
		}
		out = append(out, models...)
	}
	return out
}

func listOpenAICompatibleProviderModels(ctx context.Context, provider config.ThirdPartyProvider) ([]api.ModelInfo, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(provider.BaseURL), "/")
	if baseURL == "" || strings.TrimSpace(provider.APIKey) == "" {
		return nil, nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/models", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(provider.APIKey))

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("listing provider models failed %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var result struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	models := make([]api.ModelInfo, 0, len(result.Data))
	for _, item := range result.Data {
		modelID := strings.TrimSpace(item.ID)
		if modelID == "" {
			continue
		}
		models = append(models, api.ModelInfo{
			Name:        modelID,
			Model:       modelID,
			DisplayName: fmt.Sprintf("%s [%s]", modelID, provider.Name),
			Format:      "api",
			Source:      providerSource(provider.ID),
			PipelineTag: "text-generation",
		})
	}
	return models, nil
}

func providerEngineBaseURL(baseURL string) string {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	return strings.TrimSuffix(baseURL, "/v1")
}

func validateThirdPartyProvider(ctx context.Context, provider config.ThirdPartyProvider) (int, error) {
	if strings.TrimSpace(provider.BaseURL) == "" {
		return 0, fmt.Errorf("base_url is required")
	}
	if strings.TrimSpace(provider.APIKey) == "" {
		return 0, fmt.Errorf("api_key is required")
	}
	models, err := listOpenAICompatibleProviderModels(ctx, provider)
	if err != nil {
		return 0, fmt.Errorf("failed to fetch model list: %w", err)
	}
	if len(models) == 0 {
		return 0, fmt.Errorf("no models returned from provider")
	}
	return len(models), nil
}

func newThirdPartyProviderEngine(source, modelID string) (inference.Engine, error) {
	providerID := providerIDFromSource(source)
	provider, ok := getThirdPartyProvider(providerID)
	if !ok {
		return nil, inference.NewHTTPStatusError(http.StatusNotFound, "third-party provider not found")
	}
	baseURL := strings.TrimSpace(provider.BaseURL)
	apiKey := strings.TrimSpace(provider.APIKey)
	if baseURL == "" || apiKey == "" {
		return nil, inference.NewHTTPStatusError(http.StatusBadRequest, "third-party provider is missing base URL or API key")
	}
	return inference.NewOpenAIEngine(providerEngineBaseURL(baseURL), modelID, apiKey), nil
}
