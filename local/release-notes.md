## Changes

- Fix local ASR startup for Qwen3-ASR and GLM-ASR models by loading FunASR wrapper models from their local model directories.
- Install the `qwen-asr` runtime dependency automatically when ASR support is repaired or installed.
- Clear completed download tasks so deleted models do not reappear as stale download rows.
