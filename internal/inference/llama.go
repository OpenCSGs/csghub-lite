package inference

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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
	cmd       *exec.Cmd
	port      int
	modelPath string
	modelName string
	client    *http.Client
	logBuf    *cappedWriter
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

func newLlamaEngine(modelPath, modelName string, verbose bool) (*llamaEngine, error) {
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
		"-c", "4096",
	}

	engine.cmd = exec.Command(binary, args...)
	if verbose {
		engine.cmd.Stdout = os.Stderr
		engine.cmd.Stderr = os.Stderr
	} else {
		w := newCappedWriter(8192)
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

	if err := engine.waitForReady(30 * time.Second); err != nil {
		engine.Close()
		return nil, fmt.Errorf("llama-server failed to start: %w", err)
	}

	return engine, nil
}

func (e *llamaEngine) waitForReady(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	url := fmt.Sprintf("http://127.0.0.1:%d/health", e.port)

	for time.Now().Before(deadline) {
		resp, err := http.Get(url)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
		time.Sleep(500 * time.Millisecond)
	}

	msg := "timeout waiting for llama-server to be ready"
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

func (e *llamaEngine) Generate(ctx context.Context, prompt string, opts Options, onToken TokenCallback) (string, error) {
	messages := []Message{
		{Role: "user", Content: prompt},
	}
	return e.Chat(ctx, messages, opts, onToken)
}

func (e *llamaEngine) Chat(ctx context.Context, messages []Message, opts Options, onToken TokenCallback) (string, error) {
	if opts.MaxTokens == 0 {
		opts = DefaultOptions()
	}

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

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshaling request: %w", err)
	}

	url := e.baseURL() + "/v1/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("inference request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return "", fmt.Errorf("inference error %d: %s", resp.StatusCode, string(errBody))
	}

	if onToken != nil {
		return e.handleStream(resp.Body, onToken)
	}
	return e.handleNonStream(resp.Body)
}

func (e *llamaEngine) handleStream(body io.Reader, onToken TokenCallback) (string, error) {
	scanner := bufio.NewScanner(body)
	var full strings.Builder

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
			token := d.Content
			if token == "" {
				token = d.ReasoningContent
			}
			if token != "" {
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
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(body).Decode(&resp); err != nil {
		return "", fmt.Errorf("decoding response: %w", err)
	}
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no choices in response")
	}
	return resp.Choices[0].Message.Content, nil
}

func (e *llamaEngine) Close() error {
	if e.cmd != nil && e.cmd.Process != nil {
		e.cmd.Process.Kill()
		e.cmd.Wait()
	}
	return nil
}

func (e *llamaEngine) ModelName() string {
	return e.modelName
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
