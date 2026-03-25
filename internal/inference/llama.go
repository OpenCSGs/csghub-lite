package inference

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

// cappedWriter keeps only the last maxBytes of data written to it.
// Safe for concurrent use.
type cappedWriter struct {
	mu       sync.Mutex
	buf      bytes.Buffer
	maxBytes int
}

func newCappedWriter(maxBytes int) *cappedWriter {
	return &cappedWriter{maxBytes: maxBytes}
}

func (w *cappedWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.buf.Write(p)
	if w.buf.Len() > w.maxBytes {
		b := w.buf.Bytes()
		w.buf.Reset()
		w.buf.Write(b[len(b)-w.maxBytes:])
	}
	return len(p), nil
}

func (w *cappedWriter) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buf.String()
}

// llamaEngine manages a llama-server subprocess and communicates via its
// OpenAI-compatible HTTP API. This avoids CGO complexity while providing
// full llama.cpp inference capabilities.
type llamaEngine struct {
	cmd           *exec.Cmd
	port          int
	modelPath     string
	modelName     string
	client        *http.Client
	logBuf        *cappedWriter
	hasMultimodal bool
}

type inferenceHTTPError struct {
	status int
	body   string
}

func (e *inferenceHTTPError) Error() string {
	return fmt.Sprintf("inference error %d: %s", e.status, e.body)
}

func findLlamaBinary() string {
	// Search common names in PATH
	names := []string{"llama-server", "llama.cpp-server", "llamacpp-server"}
	for _, name := range names {
		if path, err := exec.LookPath(name); err == nil {
			return path
		}
	}
	// Check common install locations
	home, _ := os.UserHomeDir()
	locations := []string{
		"/usr/local/bin/llama-server",
		"/opt/homebrew/bin/llama-server",
	}
	if home != "" {
		locations = append(locations, home+"/bin/llama-server")
	}
	if runtime.GOOS == "windows" {
		locations = append(locations, `C:\llama.cpp\build\bin\Release\llama-server.exe`)
	}
	for _, loc := range locations {
		if _, err := os.Stat(loc); err == nil {
			return loc
		}
	}
	return ""
}

func findFreePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

// llamaReadyTimeout returns how long to wait for llama-server /health after start.
// Large GGUF files can take many minutes to mmap / load to GPU.
func llamaReadyTimeout(modelPath string) time.Duration {
	if v := strings.TrimSpace(os.Getenv("CSGHUB_LITE_LLAMA_READY_TIMEOUT")); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			return d
		}
		if sec, err := strconv.Atoi(v); err == nil && sec > 0 {
			return time.Duration(sec) * time.Second
		}
	}
	fi, err := os.Stat(modelPath)
	if err != nil {
		return 20 * time.Minute
	}
	gb := float64(fi.Size()) / (1024 * 1024 * 1024)
	// 2 min base + ~1 min per GiB (F16 9B is ~17GiB on disk → ~19 min).
	sec := int(120 + gb*60)
	if sec < 120 {
		sec = 120
	}
	if sec > 45*60 {
		sec = 45 * 60
	}
	return time.Duration(sec) * time.Second
}

// llamaContextSize returns the llama-server context window (--ctx-size / -c).
// Defaults to 8192 to better handle multimodal requests with image inputs.
// Override via CSGHUB_LITE_LLAMA_NUM_CTX.
func llamaContextSize(requested int) int {
	if requested >= 1024 {
		return requested
	}
	const defaultCtx = 8192
	if v := strings.TrimSpace(os.Getenv("CSGHUB_LITE_LLAMA_NUM_CTX")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 1024 {
			return n
		}
	}
	return defaultCtx
}

func newLlamaEngine(modelPath, modelName string, verbose bool, progress ConvertProgressFunc, numCtx int, mmproj ...string) (*llamaEngine, error) {
	binary := findLlamaBinary()
	if binary == "" {
		return nil, fmt.Errorf("llama-server not found in PATH.\n" +
			"Install llama.cpp: https://github.com/ggerganov/llama.cpp\n" +
			"  macOS:  brew install llama.cpp\n" +
			"  Linux:  build from source or use package manager\n" +
			"  Windows: download from releases page")
	}

	port, err := findFreePort()
	if err != nil {
		return nil, fmt.Errorf("finding free port: %w", err)
	}

	engine := &llamaEngine{
		port:      port,
		modelPath: modelPath,
		modelName: modelName,
		client:    &http.Client{Timeout: 0},
	}

	args := []string{
		"-m", modelPath,
		"--host", "127.0.0.1",
		"--port", fmt.Sprintf("%d", port),
		"-c", strconv.Itoa(llamaContextSize(numCtx)),
		"--reasoning", "off",
	}
	if len(mmproj) > 0 && mmproj[0] != "" {
		args = append(args, "--mmproj", mmproj[0])
		engine.hasMultimodal = true
	}
	if hasGPU() {
		args = append(args, "-ngl", "9999")
	}

	engine.cmd = exec.Command(binary, args...)
	if verbose {
		engine.cmd.Stdout = os.Stderr
		engine.cmd.Stderr = os.Stderr
	} else {
		// Large models print long tensor/KV lists; keep more tail for error diagnosis.
		w := newCappedWriter(64 * 1024)
		engine.cmd.Stdout = w
		engine.cmd.Stderr = w
		engine.logBuf = w
	}

	// Ensure shared libraries co-located with the binary can be found
	binDir := filepath.Dir(binary)
	env := os.Environ()
	switch runtime.GOOS {
	case "darwin":
		env = appendLibPath(env, "DYLD_LIBRARY_PATH", binDir)
	case "linux":
		env = appendLibPath(env, "LD_LIBRARY_PATH", binDir)
	case "windows":
		env = appendLibPath(env, "PATH", binDir)
	}
	engine.cmd.Env = env

	if err := engine.cmd.Start(); err != nil {
		return nil, fmt.Errorf("starting llama-server: %w", err)
	}

	readyTimeout := llamaReadyTimeout(modelPath)
	if progress != nil {
		progress("Starting llama-server", 0, 0)
	}
	if err := engine.waitForReady(readyTimeout, progress); err != nil {
		engine.Close()
		return nil, fmt.Errorf("llama-server failed to start: %w", err)
	}

	return engine, nil
}

func (e *llamaEngine) waitForReady(timeout time.Duration, progress ConvertProgressFunc) error {
	deadline := time.Now().Add(timeout)
	url := fmt.Sprintf("http://127.0.0.1:%d/health", e.port)

	healthClient := &http.Client{Timeout: 3 * time.Second}

	// Monitor process exit in background so we can fail fast.
	exited := make(chan error, 1)
	go func() { exited <- e.cmd.Wait() }()

	start := time.Now()
	lastBeat := time.Time{}

	for time.Now().Before(deadline) {
		if progress != nil && time.Since(lastBeat) >= 2*time.Second {
			progress("Loading model with llama-server", int(time.Since(start).Seconds()), 0)
			lastBeat = time.Now()
		}

		select {
		case err := <-exited:
			msg := "llama-server exited unexpectedly"
			if err != nil {
				msg += ": " + err.Error()
			}
			if e.logBuf != nil {
				if tail := strings.TrimSpace(e.logBuf.String()); tail != "" {
					msg += "\n\nllama-server output:\n" + tail
				}
			}
			e.cmd = nil // process already exited
			return fmt.Errorf("%s", msg)
		default:
		}

		resp, err := healthClient.Get(url)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
		time.Sleep(500 * time.Millisecond)
	}

	msg := fmt.Sprintf("timeout waiting for llama-server to be ready (waited %v; large models need more time — try CSGHUB_LITE_LLAMA_READY_TIMEOUT=45m)", timeout)
	if e.logBuf != nil {
		if tail := strings.TrimSpace(e.logBuf.String()); tail != "" {
			msg += "\n\nllama-server output:\n" + tail
		}
	}
	return fmt.Errorf("%s", msg)
}

func (e *llamaEngine) baseURL() string {
	return fmt.Sprintf("http://127.0.0.1:%d", e.port)
}

func (e *llamaEngine) ChatCompletion(ctx context.Context, reqBody map[string]interface{}) (*http.Response, error) {
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.baseURL()+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("inference request failed: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		errBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		reqDebug := string(body)
		if len(reqDebug) > 500 {
			reqDebug = reqDebug[:500] + "...(truncated)"
		}
		log.Printf("llama-server error %d: %s\nRequest (truncated): %s", resp.StatusCode, string(errBody), reqDebug)
		return nil, &inferenceHTTPError{status: resp.StatusCode, body: string(errBody)}
	}
	return resp, nil
}

func (e *llamaEngine) Generate(ctx context.Context, prompt string, opts Options, onToken TokenCallback) (string, error) {
	messages := []Message{
		{Role: "user", Content: interface{}(prompt)},
	}
	return e.Chat(ctx, messages, opts, onToken)
}

// supportedImagePrefix lists data URL prefixes that llama-server (stb_image) can decode.
var supportedImagePrefixes = []string{
	"data:image/png;base64,",
	"data:image/jpeg;base64,",
	"data:image/jpg;base64,",
	"data:image/gif;base64,",
	"data:image/bmp;base64,",
}

func isSupportedImageURL(url string) bool {
	lower := strings.ToLower(url)
	for _, prefix := range supportedImagePrefixes {
		if strings.HasPrefix(lower, prefix) {
			return true
		}
	}
	// Remote URLs (http/https) are also supported by llama-server
	return strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://")
}

// sanitizeMessages processes multimodal messages:
// - Without multimodal: strips all image_url parts, keeping only text
// - With multimodal: strips unsupported image formats (e.g. WebP, HEIC)
func (e *llamaEngine) sanitizeMessages(messages []Message) []Message {
	out := make([]Message, 0, len(messages))
	for _, m := range messages {
		parts, ok := m.Content.([]interface{})
		if !ok {
			out = append(out, m)
			continue
		}

		if !e.hasMultimodal {
			var text string
			for _, p := range parts {
				pm, _ := p.(map[string]interface{})
				if pm != nil && pm["type"] == "text" {
					if t, ok := pm["text"].(string); ok {
						text += t
					}
				}
			}
			if text == "" {
				text = "(image removed)"
			}
			out = append(out, Message{Role: m.Role, Content: text})
			continue
		}

		// Multimodal engine: keep text and supported images only
		var filtered []interface{}
		for _, p := range parts {
			pm, ok := p.(map[string]interface{})
			if !ok {
				continue
			}
			if pm["type"] == "text" {
				filtered = append(filtered, p)
				continue
			}
			if pm["type"] == "image_url" {
				imgURL, _ := pm["image_url"].(map[string]interface{})
				if imgURL != nil {
					url, _ := imgURL["url"].(string)
					if isSupportedImageURL(url) {
						filtered = append(filtered, p)
					} else {
						log.Printf("stripping unsupported image format: %s", url[:min(80, len(url))])
					}
				}
			}
		}
		if len(filtered) == 0 {
			out = append(out, Message{Role: m.Role, Content: "(images removed - unsupported format)"})
		} else {
			out = append(out, Message{Role: m.Role, Content: filtered})
		}
	}
	return out
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (e *llamaEngine) Chat(ctx context.Context, messages []Message, opts Options, onToken TokenCallback) (string, error) {
	if opts.MaxTokens == 0 {
		opts = DefaultOptions()
	}

	messages = e.sanitizeMessages(messages)

	if shouldDebugQwen35(e.modelName) {
		log.Printf("qwen35-debug request model=%q temp=%.3f top_p=%.3f max_tokens=%d messages=%d last_user=%q system=%q",
			e.modelName, opts.Temperature, opts.TopP, opts.MaxTokens, len(messages),
			lastUserText(messages), firstSystemText(messages))
	}

	for {
		respBody, err := e.chatOnce(ctx, messages, opts, onToken)
		if err == nil {
			return respBody, nil
		}
		httpErr := &inferenceHTTPError{}
		if !errors.As(err, &httpErr) || httpErr.status != http.StatusBadRequest || !strings.Contains(httpErr.body, "exceed_context_size_error") {
			return "", err
		}

		trimmed, ok := trimOldestNonSystemMessage(messages)
		if !ok {
			return "", err
		}
		log.Printf("context overflow for model %q; trimming history and retrying (%d -> %d messages)", e.modelName, len(messages), len(trimmed))
		messages = trimmed
	}
}

func (e *llamaEngine) chatOnce(ctx context.Context, messages []Message, opts Options, onToken TokenCallback) (string, error) {
	reqBody := map[string]interface{}{
		"messages":    messages,
		"temperature": opts.Temperature,
		"top_p":       opts.TopP,
		"max_tokens":  opts.MaxTokens,
		"stream":      onToken != nil,
	}
	if opts.Seed >= 0 {
		reqBody["seed"] = opts.Seed
	}
	if len(opts.Stop) > 0 {
		reqBody["stop"] = opts.Stop
	}

	resp, err := e.ChatCompletion(ctx, reqBody)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if onToken != nil {
		return e.handleStream(resp.Body, onToken)
	}
	return e.handleNonStream(resp.Body)
}

func trimOldestNonSystemMessage(messages []Message) ([]Message, bool) {
	if len(messages) == 0 {
		return nil, false
	}
	start := 0
	if messages[0].Role == "system" {
		start = 1
	}
	if len(messages)-start <= 1 {
		return nil, false
	}
	trimmed := make([]Message, 0, len(messages)-1)
	trimmed = append(trimmed, messages[:start]...)
	trimmed = append(trimmed, messages[start+1:]...)
	return trimmed, true
}

func (e *llamaEngine) handleStream(body io.Reader, onToken TokenCallback) (string, error) {
	scanner := bufio.NewScanner(body)
	var full strings.Builder
	debugCount := 0

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var chunk struct {
			Choices []struct {
				Delta struct {
					Content          string `json:"content"`
					ReasoningContent string `json:"reasoning_content"`
				} `json:"delta"`
			} `json:"choices"`
		}
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}
		if len(chunk.Choices) > 0 {
			d := chunk.Choices[0].Delta
			// Use at most one delta text per chunk. Some llama-server builds populate both
			// content and reasoning_content with the same or overlapping text; emitting both
			// caused duplicated / runaway-looking output on normal (non-reasoning) models.
			// Reasoning-first models stream with content empty until the answer — then we
			// fall back to reasoning_content.
			var token string
			switch {
			case d.Content != "":
				token = d.Content
			case d.ReasoningContent != "":
				token = d.ReasoningContent
			}
			if token != "" {
				if shouldDebugQwen35(e.modelName) && debugCount < 40 {
					debugCount++
					log.Printf("qwen35-debug chunk[%d] content=%q reasoning=%q chosen=%q",
						debugCount, d.Content, d.ReasoningContent, token)
				}
				full.WriteString(token)
				onToken(token)
			}
		}
	}

	return full.String(), scanner.Err()
}

func (e *llamaEngine) handleNonStream(body io.Reader) (string, error) {
	var resp struct {
		Choices []struct {
			Message struct {
				Content          string `json:"content"`
				ReasoningContent string `json:"reasoning_content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(body).Decode(&resp); err != nil {
		return "", fmt.Errorf("decoding response: %w", err)
	}
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no choices in response")
	}
	msg := resp.Choices[0].Message
	if msg.Content != "" && msg.ReasoningContent != "" {
		if msg.Content == msg.ReasoningContent {
			return msg.Content, nil
		}
		return msg.ReasoningContent + msg.Content, nil
	}
	if msg.Content != "" {
		return msg.Content, nil
	}
	return msg.ReasoningContent, nil
}

func shouldDebugQwen35(modelName string) bool {
	return strings.Contains(strings.ToLower(modelName), "qwen3.5")
}

func firstSystemText(messages []Message) string {
	for _, m := range messages {
		if m.Role == "system" {
			return summarizeMessageText(m.Content)
		}
	}
	return ""
}

func lastUserText(messages []Message) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" {
			return summarizeMessageText(messages[i].Content)
		}
	}
	return ""
}

func summarizeMessageText(content interface{}) string {
	switch v := content.(type) {
	case string:
		if len(v) > 120 {
			return v[:120] + "...(truncated)"
		}
		return v
	case []interface{}:
		var parts []string
		for _, item := range v {
			pm, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			if pm["type"] == "text" {
				if t, ok := pm["text"].(string); ok && t != "" {
					parts = append(parts, t)
				}
			}
		}
		s := strings.Join(parts, "")
		if len(s) > 120 {
			return s[:120] + "...(truncated)"
		}
		return s
	default:
		return fmt.Sprintf("%T", content)
	}
}

func (e *llamaEngine) Close() error {
	if e.cmd != nil && e.cmd.Process != nil {
		e.cmd.Process.Kill()
		done := make(chan struct{})
		go func() {
			e.cmd.Wait()
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			// Process stuck in uninterruptible state; abandon it.
		}
	}
	return nil
}

func (e *llamaEngine) ModelName() string {
	return e.modelName
}

func hasGPU() bool {
	if _, err := exec.LookPath("nvidia-smi"); err == nil {
		return true
	}
	if runtime.GOOS == "linux" {
		if _, err := os.Stat("/dev/kfd"); err == nil {
			return true
		}
	}
	return false
}

func appendLibPath(env []string, key, dir string) []string {
	for i, e := range env {
		if strings.HasPrefix(e, key+"=") {
			env[i] = e + string(os.PathListSeparator) + dir
			return env
		}
	}
	return append(env, key+"="+dir)
}
