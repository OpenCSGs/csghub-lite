# Bundled `convert_hf_to_gguf.py`

This file is embedded into the `csghub-lite` binary (`go:embed`) so SafeTensors → GGUF conversion does not need to download the converter script itself at runtime. `csghub-lite` fetches the matching `gguf-py` package from the Gitee `llama.cpp` source tag and adds it to `PYTHONPATH`; it does not install `gguf` from PyPI.

| Field | Value |
|-------|--------|
| Upstream tag | `b8914` (see `BundledConverterLLamacppRef` in `bundled_converter.go`) |

## Refreshing from llama.cpp

1. Sync the bundled converter from the latest llama.cpp release:

   ```bash
   source ~/.myshrc
   ./scripts/sync-llama-converter.sh
   ```

   To pin a specific release instead of the latest one:

   ```bash
   source ~/.myshrc
   ./scripts/sync-llama-converter.sh --tag b8914
   ```

2. Review the updated `convert_hf_to_gguf.py`, `bundled_converter.go`, and this README.

3. Commit the sync before creating a release tag. `scripts/push.sh` runs `./scripts/sync-llama-converter.sh --check` and refuses to publish a stale bundled converter by default.

Optional: set **`CSGHUB_LITE_CONVERTER_URL`** at runtime to a raw mirror URL instead of using the embedded file.

## Python runtime dependencies

`csghub-lite` materializes this script and runs it with an isolated Python virtual environment under `~/.csghub-lite/tools/python`. Missing runtime packages are installed automatically before conversion; setup commands are shown to users only if automatic setup fails:

| Package | Role |
|---------|------|
| `torch` | Load tensors / weights |
| `safetensors` | Read `.safetensors` checkpoints |
| `gguf` | Write GGUF; always loaded from matching `gguf-py` source extracted from `https://gitee.com/xzgan/llama.cpp` at `BundledConverterLLamacppRef`, never from PyPI |
| `transformers` | `AutoConfig`, tokenizers, etc.; if it is too old to recognize a new architecture, `csghub-lite` auto-runs `python -m pip install -U transformers` inside the managed venv and retries once |
| `sentencepiece` | Read SentencePiece tokenizers used by Qwen and similar models |

One-time install (same as the CLI error text):

```bash
python3 -m venv ~/.csghub-lite/tools/python
~/.csghub-lite/tools/python/bin/python -m pip install --upgrade --index-url https://mirrors.aliyun.com/pypi/simple pip
~/.csghub-lite/tools/python/bin/python -m pip install --index-url https://mirrors.aliyun.com/pypi/simple --find-links https://mirrors.aliyun.com/pytorch-wheels/cpu torch
~/.csghub-lite/tools/python/bin/python -m pip install --index-url https://mirrors.aliyun.com/pypi/simple safetensors transformers sentencepiece
```

Automatic setup retries the torch install with the official PyTorch CPU index (`https://download.pytorch.org/whl/cpu`) if the Aliyun mirror is unavailable.

On macOS/Linux the tool tries `python3.13` … `python3.9`, then `python3` / `python`, plus common Homebrew paths, and skips interpreters older than Python 3.9. On Windows it looks for `python` / `python3` on `PATH`.

### Optional / model-specific imports

The upstream script may import extra packages for certain architectures. These are **not** verified up front; install if conversion fails with `ModuleNotFoundError`:

| Package | When needed |
|---------|----------------|
| `numpy` | Used by the script; usually already installed with `torch` |
| `sentencepiece` | Some SentencePiece-based tokenizers |
| `huggingface_hub` | e.g. `snapshot_download` paths in the converter |
| `mistral_common` | Some Mistral flows; upstream suggests `pip install mistral-common[image,audio]` |
