# csghub-lite

A lightweight tool for running large language models locally, powered by models from the [CSGHub](https://opencsg.com) platform.

Inspired by [Ollama](https://ollama.com), csghub-lite provides model download, local inference, interactive chat, and an OpenAI-compatible REST API — all from a single binary.

## Features

- **One command to start** — `csghub-lite run` downloads, loads, and chats
- **Model keep-alive** — models stay loaded after exit (default 5 min), instant reconnect
- **Auto-start server** — background API server starts automatically, no manual setup
- **Model download** from CSGHub platform (hub.opencsg.com or private deployments)
- **Local inference** via llama.cpp (GGUF models, SafeTensors auto-converted)
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

## Quick Start

```bash
# Run a model — downloads, starts server, and chats (all automatic)
csghub-lite run Qwen/Qwen3-0.6B-GGUF

# Keep a model loaded until you stop it manually
csghub-lite run Qwen/Qwen3-0.6B-GGUF --keep-alive -1

# Search for models on CSGHub
csghub-lite search "qwen"

# Check running models (model stays loaded after exit)
csghub-lite ps

# Set CSGHub access token (optional, for private models)
csghub-lite login
```

> **Note:** The install script automatically installs [llama-server](https://github.com/ggml-org/llama.cpp) (required for inference). If you installed from source, install it separately: `brew install llama.cpp` (macOS) or download from [llama.cpp releases](https://github.com/ggml-org/llama.cpp/releases).

## CLI Commands

| Command | Description |
|---|---|
| `csghub-lite run <model>` | Pull, start server, and chat (all automatic) |
| `csghub-lite chat <model>` | Chat with a locally downloaded model |
| `csghub-lite ps` | List currently running models and their keep-alive |
| `csghub-lite stop <model>` | Stop/unload a running model |
| `csghub-lite serve` | Start the API server (auto-started by `run`) |
| `csghub-lite pull <model>` | Download a model from CSGHub |
| `csghub-lite list` / `ls` | List locally downloaded models |
| `csghub-lite show <model>` | Show model details (format, size, files) |
| `csghub-lite rm <model>` | Remove a locally downloaded model |
| `csghub-lite login` | Set CSGHub access token |
| `csghub-lite search <query>` | Search models on CSGHub |
| `csghub-lite config set <key> <value>` | Set configuration |
| `csghub-lite config get <key>` | Get a configuration value |
| `csghub-lite config show` | Show current configuration |
| `csghub-lite uninstall` | Remove csghub-lite and llama-server, preserving local data unless `--all` is set |
| `csghub-lite --version` | Show version information |

Model names use the format `namespace/name`, e.g. `Qwen/Qwen3-0.6B-GGUF`.

### run vs chat

- **`run`** — Downloads the model if not present, auto-starts the background server, and opens a chat session. After you exit, the model stays loaded for 5 minutes by default so the next `run` is instant. Use `--keep-alive -1` to keep it loaded until you stop it manually.
- **`chat`** — Starts a chat session with a model that is already downloaded. Supports `--system` flag for custom system prompts.

```bash
# Auto-download and chat (first time)
csghub-lite run Qwen/Qwen3-0.6B-GGUF

# Exit chat, model stays loaded — reconnect instantly
csghub-lite run Qwen/Qwen3-0.6B-GGUF

# Keep the model loaded until `csghub-lite stop`
csghub-lite run Qwen/Qwen3-0.6B-GGUF --keep-alive -1

# Check which models are still loaded
csghub-lite ps

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

For full endpoint details and examples, see the [REST API Reference](docs/api/overview.md).

If the local server is running, you can also open the interactive API docs in your browser:
[http://localhost:11435/api-docs.html](http://localhost:11435/api-docs.html)

## Logs

By default, csghub-lite writes logs under `~/.csghub-lite/logs/`:

- `csghub-lite.log` — API server logs
- `llama-server.log` — llama-server subprocess logs

## Configuration

Configuration is stored at `~/.csghub-lite/config.json`.

The CLI and Web UI expose a convenience `storage_dir` setting. When you set it, csghub-lite expands it into the persisted `model_dir` and `dataset_dir`.

| Key | Default | Description |
|---|---|---|
| `storage_dir` | `~/.csghub-lite` | Shared local storage root for models and datasets |
| `server_url` | `https://hub.opencsg.com` | CSGHub platform URL |
| `ai_gateway_url` | `https://ai.space.opencsg.com` | AI Gateway URL for cloud inference models |
| `model_dir` | `~/.csghub-lite/models` | Effective local model storage directory |
| `dataset_dir` | `~/.csghub-lite/datasets` | Effective local dataset storage directory |
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
| SafeTensors | Yes | Yes (auto-converted to GGUF) |

SafeTensors checkpoints are converted once using the bundled llama.cpp `convert_hf_to_gguf.py` and **system Python** (PyTorch is not shipped inside the release binary). Install these packages once:

```bash
pip3 install --index-url https://download.pytorch.org/whl/cpu torch
pip3 install safetensors gguf transformers
```

Use Python 3.10+ on `PATH` (Windows: `python` or `python3`). If `gguf` is too old for the bundled converter, `csghub-lite` now prefers the matching `gguf-py` from the `llama.cpp` source tag (`CSGHUB_LITE_REGION=CN` prefers `https://gitee.com/xzgan/llama.cpp`, other regions prefer GitHub); only if that repository repair fails does it fall back to `python -m pip install -U gguf` before retrying once automatically. If `transformers` is too old for a new architecture, `csghub-lite` still tries `python -m pip install -U transformers` before retrying. Some models may need extra packages (for example `sentencepiece`); see [`internal/convert/data/README.md`](internal/convert/data/README.md) for the full list and troubleshooting (`gguf` version mismatch, optional `CSGHUB_LITE_CONVERTER_URL`).

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
