package convert

import _ "embed"

//go:embed data/convert_hf_to_gguf.py
var bundledConverterPy []byte

// bundledConverterRevision must be incremented whenever data/convert_hf_to_gguf.py
// changes so upgraded binaries rewrite the cached script under ~/.csghub-lite/tools/.
const bundledConverterRevision = 4

// BundledConverterLLamacppRef documents the llama.cpp release tag the bundled script was taken from.
const BundledConverterLLamacppRef = "b8797"
