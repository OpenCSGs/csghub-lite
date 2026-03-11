package inference

// Options controls generation parameters.
type Options struct {
	Temperature float64
	TopP        float64
	TopK        int
	MaxTokens   int
	Seed        int
	NumCtx      int
	Stop        []string
}

// DefaultOptions returns sensible defaults.
func DefaultOptions() Options {
	return Options{
		Temperature: 0.7,
		TopP:        0.9,
		TopK:        40,
		MaxTokens:   2048,
		Seed:        -1,
		NumCtx:      4096,
	}
}

// Message represents a chat message.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// TokenCallback is called for each generated token during streaming.
type TokenCallback func(token string)
