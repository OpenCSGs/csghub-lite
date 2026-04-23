# Bundled `convert_hf_to_gguf.py`

This file is embedded into the `csghub-lite` binary (`go:embed`) so SafeTensors → GGUF conversion does not need to download the converter script itself at runtime. If the system `gguf` package is too old, `csghub-lite` may still fetch matching `gguf-py` from the configured `llama.cpp` source mirror.

| Field | Value |
|-------|--------|
| Upstream tag | `b8797` (see `BundledConverterLLamacppRef` in `bundled_converter.go`) |

## Refreshing from llama.cpp

1. Sync the bundled converter from the latest llama.cpp release:

   ```bash
   source ~/.myshrc
   ./scripts/sync-llama-converter.sh
   ```

   To pin a specific release instead of the latest one:

   ```bash
   source ~/.myshrc
   ./scripts/sync-llama-converter.sh --tag b8797
   ```

2. Review the updated `convert_hf_to_gguf.py`, `bundled_converter.go`, and this README.

3. Commit the sync before creating a release tag. `scripts/push.sh` runs `./scripts/sync-llama-converter.sh --check` and refuses to publish a stale bundled converter by default.

Optional: set **`CSGHUB_LITE_CONVERTER_URL`** at runtime to a raw mirror URL instead of using the embedded file.

## Python runtime dependencies

`csghub-lite` materializes this script and runs it with a system **Python 3** interpreter. The binary pre-checks the core imports before conversion (`internal/convert/convert_python.go`); `gguf` can come either from the system Python environment or from a matching `gguf-py` fetch when auto-repair is needed:

| Package | Role |
|---------|------|
| `torch` | Load tensors / weights |
| `safetensors` | Read `.safetensors` checkpoints |
| `gguf` | Write GGUF; if it is too old for the bundled converter, `csghub-lite` fetches matching `gguf-py` from the `llama.cpp` source tag and retries once (`CSGHUB_LITE_REGION=CN` prefers `https://gitee.com/xzgan/llama.cpp`, other regions prefer GitHub) |
| `transformers` | `AutoConfig`, tokenizers, etc.; if it is too old to recognize a new architecture, `csghub-lite` auto-runs `python -m pip install -U transformers` and retries once |

One-time install (same as the CLI error text):

```bash
pip3 install --index-url https://download.pytorch.org/whl/cpu torch
pip3 install safetensors gguf transformers
```

On macOS/Linux the tool tries `python3.13` … `python3.10`, then `python3` / `python`, plus common Homebrew paths. On Windows it looks for `python` / `python3` on `PATH`.

### Optional / model-specific imports

The upstream script may import extra packages for certain architectures. These are **not** verified up front; install if conversion fails with `ModuleNotFoundError`:

| Package | When needed |
|---------|----------------|
| `numpy` | Used by the script; usually already installed with `torch` |
| `sentencepiece` | Some SentencePiece-based tokenizers |
| `huggingface_hub` | e.g. `snapshot_download` paths in the converter |
| `mistral_common` | Some Mistral flows; upstream suggests `pip install mistral-common[image,audio]` |
