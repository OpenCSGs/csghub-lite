## Changes

- Fix Marketplace model details so locally downloaded models such as `openai/openai_whisper-large-v3` are shown as downloaded.
- Avoid downloading redundant Transformer weight files when SafeTensors weights are available, and skip unused FP32 PyTorch shard files.
