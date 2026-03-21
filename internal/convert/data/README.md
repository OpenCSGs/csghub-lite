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
