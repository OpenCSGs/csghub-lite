# csghub-lite

A lightweight tool for running large language models locally, powered by models from the [CSGHub](https://hub.opencsg.com) platform.

Inspired by [Ollama](https://ollama.com), csghub-lite provides model download, local inference, interactive chat, and an OpenAI-compatible REST API — all from a single binary.

## Features

- **Model download** from CSGHub platform (hub.opencsg.com or private deployments)
- **Local inference** via llama.cpp (GGUF models)
- **Interactive chat** with streaming output
- **REST API** compatible with Ollama's API format
- **Cross-platform** — macOS, Linux, Windows
- **Resume downloads** — interrupted downloads resume where they left off

## Installation

### Quick install (Linux / macOS)

```bash
curl -fsSL https://hub.opencsg.com/csghub-lite/install.sh | sh
```

### Quick install (Windows PowerShell)

```powershell
irm https://hub.opencsg.com/csghub-lite/install.ps1 | iex
```

### From source

```bash
git clone https://github.com/opencsgs/csghub-lite.git
cd csghub-lite
make build
# Binary is at bin/csghub-lite
```

### From GitHub Releases

Download the latest binary for your platform from the [Releases](https://github.com/opencsgs/csghub-lite/releases) page.

## Prerequisites

**llama-server** (from [llama.cpp](https://github.com/ggml-org/llama.cpp)) is required for model inference:

- macOS: `brew install llama.cpp`
- Linux: download from [llama.cpp releases](https://github.com/ggml-org/llama.cpp/releases)
- Windows: download from [llama.cpp releases](https://github.com/ggml-org/llama.cpp/releases)

## Quick Start

```bash
# Set your CSGHub access token (optional, for private models)
csghub-lite login

# Search for models
csghub-lite search "qwen"

# Run a model (auto-downloads if not present)
csghub-lite run Qwen/Qwen3-0.6B-GGUF

# Start the API server
csghub-lite serve
```

## CLI Commands

| Command | Description |
|---|---|
| `csghub-lite serve` | Start the local API server (default port 11435) |
| `csghub-lite run <model>` | Pull (if needed) and chat interactively |
| `csghub-lite chat <model>` | Chat with a locally downloaded model |
| `csghub-lite pull <model>` | Download a model from CSGHub |
| `csghub-lite list` / `ls` | List locally downloaded models |
| `csghub-lite show <model>` | Show model details (format, size, files) |
| `csghub-lite ps` | List currently running models on the server |
| `csghub-lite stop <model>` | Stop/unload a running model from the server |
| `csghub-lite rm <model>` | Remove a locally downloaded model |
| `csghub-lite login` | Set CSGHub access token |
| `csghub-lite search <query>` | Search models on CSGHub |
| `csghub-lite config set <key> <value>` | Set configuration |
| `csghub-lite config get <key>` | Get a configuration value |
| `csghub-lite config show` | Show current configuration |
| `csghub-lite --version` | Show version information |

Model names use the format `namespace/name`, e.g. `Qwen/Qwen3-0.6B-GGUF`.

### run vs chat

- **`run`** — Downloads the model automatically if not present, then starts a chat session. Best for first-time use.
- **`chat`** — Starts a chat session with a model that is already downloaded. Supports `--system` flag for custom system prompts.

```bash
# Auto-download and chat
csghub-lite run Qwen/Qwen3-0.6B-GGUF

# Chat with custom system prompt (model must be downloaded)
csghub-lite chat Qwen/Qwen3-0.6B-GGUF --system "You are a coding assistant."
```

### Interactive Chat Commands

Once in a chat session (`run` or `chat`):

| Command | Description |
|---|---|
| `/bye`, `/exit`, `/quit` | Exit the chat |
| `/clear` | Clear conversation context |
| `/help` | Show help |

End a line with `\` for multiline input. Press Ctrl+D to exit.

## REST API

The server listens on `localhost:11435` by default.

### Endpoints

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/health` | Health check |
| `GET` | `/api/tags` | List local models |
| `GET` | `/api/ps` | List running models |
| `POST` | `/api/show` | Show model details |
| `POST` | `/api/pull` | Pull a model (streaming) |
| `POST` | `/api/stop` | Stop/unload a running model |
| `DELETE` | `/api/delete` | Delete a model |
| `POST` | `/api/generate` | Text generation (streaming) |
| `POST` | `/api/chat` | Chat completions (streaming) |

### Example: Chat

```bash
curl http://localhost:11435/api/chat -d '{
  "model": "Qwen/Qwen3-0.6B-GGUF",
  "messages": [{"role": "user", "content": "Hello!"}]
}'
```

### Example: Generate (non-streaming)

```bash
curl http://localhost:11435/api/generate -d '{
  "model": "Qwen/Qwen3-0.6B-GGUF",
  "prompt": "Write a haiku about programming",
  "stream": false
}'
```

### Example: List running models

```bash
curl http://localhost:11435/api/ps
```

### Example: Stop a model

```bash
curl -X POST http://localhost:11435/api/stop -d '{"model": "Qwen/Qwen3-0.6B-GGUF"}'
```

## Configuration

Configuration is stored at `~/.csghub-lite/config.json`.

| Key | Default | Description |
|---|---|---|
| `server_url` | `https://hub.opencsg.com` | CSGHub platform URL |
| `model_dir` | `~/.csghub-lite/models` | Local model storage directory |
| `listen_addr` | `:11435` | API server listen address |
| `token` | (none) | CSGHub access token |

Switch to a private CSGHub deployment:

```bash
csghub-lite config set server_url https://my-private-csghub.example.com
```

## Model Formats

| Format | Download | Inference |
|---|---|---|
| GGUF | Yes | Yes (via llama.cpp) |
| SafeTensors | Yes | No (convert to GGUF first) |

To convert SafeTensors to GGUF, use llama.cpp's conversion tool:

```bash
python convert_hf_to_gguf.py /path/to/safetensors/model
```

## Development

```bash
# Run tests
make test

# Run tests with coverage
make test-cover

# Build for all platforms
make build-all

# Test goreleaser locally (no publish)
make release-snapshot

# Lint
make lint
```

## Documentation

Full documentation is available in the [`docs/`](docs/) directory:

- **Getting Started**: [Installation](docs/getting-started/installation.md) | [Quick Start](docs/getting-started/quickstart.md)
- **CLI Reference**: [All Commands](docs/cli/overview.md)
- **REST API**: [API Reference](docs/api/overview.md)
- **Guides**: [Configuration](docs/guides/configuration.md) | [Model Formats](docs/guides/model-formats.md) | [Packaging](docs/guides/packaging.md) | [Architecture](docs/guides/architecture.md)

## License

Apache-2.0
