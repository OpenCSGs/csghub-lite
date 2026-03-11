package api

import "time"

// -- Request types --

type GenerateRequest struct {
	Model  string         `json:"model"`
	Prompt string         `json:"prompt"`
	Stream *bool          `json:"stream,omitempty"`
	Options *ModelOptions `json:"options,omitempty"`
}

type ChatRequest struct {
	Model    string         `json:"model"`
	Messages []Message      `json:"messages"`
	Stream   *bool          `json:"stream,omitempty"`
	Options  *ModelOptions  `json:"options,omitempty"`
}

type PullRequest struct {
	Model string `json:"model"`
}

type DeleteRequest struct {
	Model string `json:"model"`
}

type ShowRequest struct {
	Model string `json:"model"`
}

type StopRequest struct {
	Model string `json:"model"`
}

// -- Response types --

type GenerateResponse struct {
	Model     string    `json:"model"`
	Response  string    `json:"response"`
	Done      bool      `json:"done"`
	CreatedAt time.Time `json:"created_at"`
}

type ChatResponse struct {
	Model     string    `json:"model"`
	Message   *Message  `json:"message,omitempty"`
	Done      bool      `json:"done"`
	CreatedAt time.Time `json:"created_at"`
}

type TagsResponse struct {
	Models []ModelInfo `json:"models"`
}

type ShowResponse struct {
	ModelFile  string     `json:"modelfile"`
	Details   ModelInfo   `json:"details"`
}

type PullResponse struct {
	Status    string `json:"status"`
	Digest    string `json:"digest,omitempty"`
	Total     int64  `json:"total,omitempty"`
	Completed int64  `json:"completed,omitempty"`
}

type PsResponse struct {
	Models []RunningModel `json:"models"`
}

type RunningModel struct {
	Name      string    `json:"name"`
	Model     string    `json:"model"`
	Size      int64     `json:"size"`
	Format    string    `json:"format"`
	ExpiresAt time.Time `json:"expires_at"`
}

// -- Shared types --

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ModelInfo struct {
	Name       string    `json:"name"`
	Model      string    `json:"model"`
	Size       int64     `json:"size"`
	Format     string    `json:"format"`
	ModifiedAt time.Time `json:"modified_at"`
}

type ModelOptions struct {
	Temperature float64 `json:"temperature,omitempty"`
	TopP        float64 `json:"top_p,omitempty"`
	TopK        int     `json:"top_k,omitempty"`
	MaxTokens   int     `json:"max_tokens,omitempty"`
	Seed        int     `json:"seed,omitempty"`
	NumCtx      int     `json:"num_ctx,omitempty"`
}
