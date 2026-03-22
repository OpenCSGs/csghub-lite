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

type LoadRequest struct {
	Model  string `json:"model"`
	Stream *bool  `json:"stream,omitempty"`
}

type LoadResponse struct {
	Status    string `json:"status"`
	Step      string `json:"step,omitempty"`
	Current   int    `json:"current,omitempty"`
	Total     int    `json:"total,omitempty"`
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
	Role    string      `json:"role"`
	Content interface{} `json:"content"`
}

type ModelInfo struct {
	Name        string    `json:"name"`
	Model       string    `json:"model"`
	Size        int64     `json:"size"`
	Format      string    `json:"format"`
	ModifiedAt  time.Time `json:"modified_at"`
	PipelineTag string    `json:"pipeline_tag,omitempty"`
	HasMMProj   bool      `json:"has_mmproj,omitempty"`
}

type ModelOptions struct {
	Temperature float64 `json:"temperature,omitempty"`
	TopP        float64 `json:"top_p,omitempty"`
	TopK        int     `json:"top_k,omitempty"`
	MaxTokens   int     `json:"max_tokens,omitempty"`
	Seed        int     `json:"seed,omitempty"`
	NumCtx      int     `json:"num_ctx,omitempty"`
}

// -- Dataset request types --

type DatasetPullRequest struct {
	Dataset string `json:"dataset"`
}

type DatasetDeleteRequest struct {
	Dataset string `json:"dataset"`
}

type DatasetShowRequest struct {
	Dataset string `json:"dataset"`
}

// -- Dataset response types --

type DatasetInfo struct {
	Name       string    `json:"name"`
	Dataset    string    `json:"dataset"`
	Size       int64     `json:"size"`
	Files      int       `json:"files"`
	ModifiedAt time.Time `json:"modified_at"`
}

type DatasetTagsResponse struct {
	Datasets []DatasetInfo `json:"datasets"`
}

type DatasetShowResponse struct {
	Details DatasetInfo `json:"details"`
	Files   []string    `json:"files,omitempty"`
}

type DatasetFilesRequest struct {
	Dataset string `json:"dataset"`
	Path    string `json:"path"`
}

type DatasetFileEntry struct {
	Name       string    `json:"name"`
	Size       int64     `json:"size"`
	IsDir      bool      `json:"is_dir"`
	ModifiedAt time.Time `json:"modified_at"`
}

type DatasetFilesResponse struct {
	Dataset string             `json:"dataset"`
	Path    string             `json:"path"`
	Entries []DatasetFileEntry `json:"entries"`
}

type DatasetPullResponse struct {
	Status    string `json:"status"`
	Digest    string `json:"digest,omitempty"`
	Total     int64  `json:"total,omitempty"`
	Completed int64  `json:"completed,omitempty"`
}

// -- OpenAI-compatible types --

type OpenAIChatRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Stream      *bool     `json:"stream,omitempty"`
	Temperature *float64  `json:"temperature,omitempty"`
	TopP        *float64  `json:"top_p,omitempty"`
	MaxTokens   *int      `json:"max_tokens,omitempty"`
	NumCtx      *int      `json:"num_ctx,omitempty"`
	Seed        *int      `json:"seed,omitempty"`
	Stop        []string  `json:"stop,omitempty"`
}

type OpenAIChatResponse struct {
	ID      string         `json:"id"`
	Object  string         `json:"object"`
	Created int64          `json:"created"`
	Model   string         `json:"model"`
	Choices []OpenAIChoice `json:"choices"`
	Usage   OpenAIUsage    `json:"usage"`
}

type OpenAIChoice struct {
	Index        int      `json:"index"`
	Message      *Message `json:"message,omitempty"`
	Delta        *Message `json:"delta,omitempty"`
	FinishReason *string  `json:"finish_reason"`
}

type OpenAIUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type OpenAIModelList struct {
	Object string        `json:"object"`
	Data   []OpenAIModel `json:"data"`
}

type OpenAIModel struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}
