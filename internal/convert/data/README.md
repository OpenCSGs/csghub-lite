# Bundled `convert_hf_to_gguf.py`

This file is embedded into the `csghub-lite` binary (`go:embed`) so SafeTensors → GGUF conversion does not require downloading from GitHub at runtime.

| Field | Value |
|-------|--------|
| Upstream tag | `b8350` (see `BundledConverterLLamacppRef` in `bundled_converter.go`) |

## Refreshing from llama.cpp

1. Replace `convert_hf_to_gguf.py` from the desired tag:

   ```bash
   curl -fsSL -o internal/convert/data/convert_hf_to_gguf.py \
     "https://raw.githubusercontent.com/ggml-org/llama.cpp/b8350/convert_hf_to_gguf.py"
   ```

2. Increment **`bundledConverterRevision`** in `../bundled_converter.go`.

3. Update **`BundledConverterLLamacppRef`** and the table above.

Optional: set **`CSGHUB_LITE_CONVERTER_URL`** at runtime to a raw mirror URL instead of using the embedded file.

## Python runtime dependencies

`csghub-lite` materializes this script and runs it with a system **Python 3** interpreter. The binary pre-checks imports before conversion (`internal/convert/convert_python.go`); all of the following must be importable:

| Package | Role |
|---------|------|
| `torch` | Load tensors / weights |
| `safetensors` | Read `.safetensors` checkpoints |
| `gguf` | Write GGUF; if the script fails with `AttributeError` involving `MODEL_ARCH` or `gguf`, upgrade: `pip3 install -U gguf` |
| `transformers` | `AutoConfig`, tokenizers, etc. |

One-time install (same as the CLI error text):

```bash
pip3 install torch safetensors gguf transformers
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
