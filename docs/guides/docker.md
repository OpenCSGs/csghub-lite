# Docker Runtime Images

csghub-lite Docker images are bootstrap runtimes. They include the OS and GPU
runtime dependencies needed to start, then download `csghub-lite` and
`llama-server` on container startup according to environment variables.

Persist `/root/.csghub-lite` to keep the installed binaries, downloaded models,
configuration, credentials, logs, and usage data across container restarts.

## Images

| Image | Purpose |
| --- | --- |
| `opencsg-registry.cn-beijing.cr.aliyuncs.com/opencsghq/csghub-lite:latest` | Standard Linux runtime |
| `opencsg-registry.cn-beijing.cr.aliyuncs.com/opencsghq/csghub-lite-rocm:latest` | AMD GPU hosts with ROCm |

## Install Policy

The default policy is `if-missing`: install on first start, then reuse the
persisted binaries on later starts.

| Environment variable | Description |
| --- | --- |
| `CSGHUB_LITE_VERSION` | Pin the csghub-lite version, for example `v0.5.10`. |
| `CSGHUB_LITE_LLAMA_CPP_TAG` | Pin the llama.cpp engine tag, for example `b8914`. |
| `CSGHUB_LITE_INSTALL_POLICY` | `if-missing`, `if-version-mismatch`, or `always`. |
| `CSGHUB_LITE_INSTALL_ALWAYS` | Backward-compatible shortcut for forcing reinstall on startup. |
| `CSGHUB_LITE_INSTALL_URL` | Override the installer URL for private mirrors. |
| `CSGHUB_LITE_REGION` | Force the installer region, for example `CN` or `INTL`. |
| `CSGHUB_LITE_REQUIRE_LLAMA_SERVER` | Set to `0` to allow cloud-only use without a local `llama-server`. |

## Examples

Standard runtime:

```bash
docker run -d --name csghub-lite \
  -p 11435:11435 \
  -v csghub-lite-data:/root/.csghub-lite \
  opencsg-registry.cn-beijing.cr.aliyuncs.com/opencsghq/csghub-lite:latest
```

Pin both runtime versions:

```bash
docker run -d --name csghub-lite \
  -p 11435:11435 \
  -e CSGHUB_LITE_VERSION=v0.5.10 \
  -e CSGHUB_LITE_LLAMA_CPP_TAG=b8914 \
  -e CSGHUB_LITE_INSTALL_POLICY=if-version-mismatch \
  -v csghub-lite-data:/root/.csghub-lite \
  opencsg-registry.cn-beijing.cr.aliyuncs.com/opencsghq/csghub-lite:latest
```

Force an upgrade on the next start:

```bash
docker run -d --name csghub-lite \
  -p 11435:11435 \
  -e CSGHUB_LITE_INSTALL_ALWAYS=1 \
  -v csghub-lite-data:/root/.csghub-lite \
  opencsg-registry.cn-beijing.cr.aliyuncs.com/opencsghq/csghub-lite:latest
```

ROCm runtime:

```bash
docker run -d --name csghub-lite-rocm \
  --device=/dev/kfd \
  --device=/dev/dri \
  --group-add video \
  --ipc=host \
  --security-opt seccomp=unconfined \
  -p 11435:11435 \
  -v csghub-lite-data:/root/.csghub-lite \
  opencsg-registry.cn-beijing.cr.aliyuncs.com/opencsghq/csghub-lite-rocm:latest
```
