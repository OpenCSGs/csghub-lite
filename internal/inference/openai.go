package inference

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/opencsgs/csghub-lite/pkg/api"
)

type openAIEngine struct {
	baseURL   string
	modelName string
	token     string
	client    *http.Client
}

func NewOpenAIEngine(baseURL, modelName, token string) Engine {
	return &openAIEngine{
		baseURL:   strings.TrimRight(baseURL, "/"),
		modelName: modelName,
		token:     strings.TrimSpace(token),
		client:    &http.Client{Timeout: 0},
	}
}

func (e *openAIEngine) ChatCompletion(ctx context.Context, reqBody map[string]interface{}) (*http.Response, error) {
	reqBody = sanitizeOpenAIRequestBody(e.modelName, reqBody)
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.baseURL+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if e.token != "" {
		req.Header.Set("Authorization", "Bearer "+e.token)
	}

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("chat completion request failed: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		return nil, decodeOpenAIHTTPError(resp)
	}
	return resp, nil
}

func (e *openAIEngine) Generate(ctx context.Context, prompt string, opts Options, onToken TokenCallback) (string, error) {
	messages := []Message{{Role: "user", Content: prompt}}
	return e.Chat(ctx, messages, opts, onToken)
}

func (e *openAIEngine) Chat(ctx context.Context, messages []Message, opts Options, onToken TokenCallback) (string, error) {
	stream := onToken != nil
	topK := opts.TopK
	if topK <= 0 || topK == DefaultOptions().TopK {
		topK = 10
	}
	reqBody := map[string]interface{}{
		"model":              e.modelName,
		"messages":           messagesToOpenAI(messages),
		"temperature":        opts.Temperature,
		"top_p":              opts.TopP,
		"top_k":              topK,
		"repetition_penalty": 1,
		"max_tokens":         opts.MaxTokens,
		"stream":             stream,
	}
	if opts.Seed >= 0 {
		reqBody["seed"] = opts.Seed
	}
	if len(opts.Stop) > 0 {
		reqBody["stop"] = opts.Stop
	}
	reqBody = sanitizeOpenAIRequestBody(e.modelName, reqBody)

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshaling request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.baseURL+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if stream {
		req.Header.Set("Accept", "text/event-stream")
	} else {
		req.Header.Set("Accept", "application/json")
	}
	if e.token != "" {
		req.Header.Set("Authorization", "Bearer "+e.token)
	}

	resp, err := e.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("chat request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", decodeOpenAIHTTPError(resp)
	}

	if stream {
		return e.handleStream(resp.Body, onToken)
	}
	return e.handleJSONResponse(resp.Body)
}

func (e *openAIEngine) handleStream(body io.Reader, onToken TokenCallback) (string, error) {
	scanner := bufio.NewScanner(body)
	var full strings.Builder

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data: "))
		if data == "" {
			continue
		}
		if data == "[DONE]" {
			break
		}

		var chatResp api.OpenAIChatResponse
		if err := json.Unmarshal([]byte(data), &chatResp); err != nil {
			continue
		}
		if len(chatResp.Choices) == 0 || chatResp.Choices[0].Delta == nil {
			continue
		}

		if token := openAIContentString(chatResp.Choices[0].Delta.Content); token != "" {
			full.WriteString(token)
			onToken(token)
		}
	}

	return full.String(), scanner.Err()
}

func (e *openAIEngine) handleJSONResponse(body io.Reader) (string, error) {
	var chatResp api.OpenAIChatResponse
	if err := json.NewDecoder(body).Decode(&chatResp); err != nil {
		return "", fmt.Errorf("decoding response: %w", err)
	}
	if len(chatResp.Choices) == 0 || chatResp.Choices[0].Message == nil {
		return "", fmt.Errorf("no message in response")
	}
	return openAIContentString(chatResp.Choices[0].Message.Content), nil
}

func (e *openAIEngine) Close() error {
	return nil
}

func (e *openAIEngine) ModelName() string {
	return e.modelName
}

func decodeOpenAIHTTPError(resp *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
	message := strings.TrimSpace(string(body))
	if len(body) > 0 {
		var payload struct {
			Error interface{} `json:"error"`
			Msg   string      `json:"msg"`
		}
		if err := json.Unmarshal(body, &payload); err == nil {
			switch v := payload.Error.(type) {
			case string:
				if strings.TrimSpace(v) != "" {
					message = strings.TrimSpace(v)
				}
			case map[string]interface{}:
				if msg, ok := v["message"].(string); ok && strings.TrimSpace(msg) != "" {
					message = strings.TrimSpace(msg)
				}
			}
			if strings.TrimSpace(payload.Msg) != "" {
				message = strings.TrimSpace(payload.Msg)
			}
		}
	}
	if message == "" {
		message = resp.Status
	}
	return NewHTTPStatusError(resp.StatusCode, message)
}

func sanitizeOpenAIRequestBody(modelName string, reqBody map[string]interface{}) map[string]interface{} {
	if len(reqBody) == 0 {
		return reqBody
	}
	if !openAIModelRequiresSingleSamplingParam(modelName) {
		return reqBody
	}
	if _, hasTemp := reqBody["temperature"]; !hasTemp {
		return reqBody
	}
	if _, hasTopP := reqBody["top_p"]; !hasTopP {
		return reqBody
	}

	out := make(map[string]interface{}, len(reqBody))
	for key, value := range reqBody {
		out[key] = value
	}
	delete(out, "top_p")
	return out
}

func openAIModelRequiresSingleSamplingParam(modelName string) bool {
	modelName = strings.TrimSpace(strings.ToLower(modelName))
	return strings.HasPrefix(modelName, "claude")
}

func messagesToOpenAI(messages []Message) []map[string]interface{} {
	out := make([]map[string]interface{}, 0, len(messages))
	for _, msg := range messages {
		out = append(out, map[string]interface{}{
			"role":    msg.Role,
			"content": msg.Content,
		})
	}
	return out
}

func openAIContentString(content interface{}) string {
	switch v := content.(type) {
	case nil:
		return ""
	case string:
		return v
	default:
		buf, err := json.Marshal(v)
		if err != nil {
			return fmt.Sprintf("%v", v)
		}
		return string(buf)
	}
}
