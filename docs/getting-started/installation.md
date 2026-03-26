# 安装指南

## 系统要求

- **操作系统**: macOS (Apple Silicon / Intel)、Linux (x86_64 / ARM64)、Windows (x86_64)
- **推理依赖**: [llama-server](https://github.com/ggml-org/llama.cpp)（模型推理必需）
- **编译依赖**: Go 1.22+（仅源码编译需要）

## 安装 csghub-lite

### 方式一：一键安装脚本（推荐）

适用于 Linux 和 macOS，自动检测系统架构，从 GitHub Releases 下载安装。

```bash
curl -fsSL https://hub.opencsg.com/csghub-lite/install.sh | sh
```

指定版本安装：

```bash
CSGHUB_LITE_VERSION=v0.1.0 curl -fsSL https://hub.opencsg.com/csghub-lite/install.sh | sh
```

安装脚本环境变量（可选）：

| 变量 | 说明 |
|------|------|
| `CSGHUB_LITE_AUTO_INSTALL_LLAMA_SERVER` | 设为 `0` 可跳过自动安装/升级 `llama-server`。 |
| `CSGHUB_LITE_AUTO_INSTALL_PATCHELF` | Linux 上设为 `0` 可禁止自动 `apt/dnf/yum install patchelf`（用于为 `llama-server` 设置 `$ORIGIN`，使同目录 `.so` 可被直接加载）。 |

说明：若远程 llama.cpp 与本地 **build 号一致**，脚本会跳过重新下载；此前若因缺少 `libmtmd.so.0` 等导致 `llama-server --version` 失败，会被误判为需要升级——新版本已用 `LD_LIBRARY_PATH` 检测版本，并从压缩包 **递归** 安装所有 `.so`。

### 方式二：Homebrew（主要面向 macOS）

可选额外入口，主安装入口仍然推荐使用上面的 `curl ... | sh`。Linux 仍建议优先使用安装脚本、release 压缩包或系统包管理器。

```bash
brew tap opencsgs/csghub-lite https://github.com/OpenCSGs/csghub-lite
brew install opencsgs/csghub-lite/csghub-lite
```

### 方式三：GitHub Releases 手动下载

前往 [Releases](https://github.com/opencsgs/csghub-lite/releases) 页面，下载对应平台的压缩包：

| 平台 | 文件名 |
|------|--------|
| macOS Apple Silicon | `csghub-lite_*_darwin_arm64.tar.gz` |
| macOS Intel | `csghub-lite_*_darwin_amd64.tar.gz` |
| Linux x86_64 | `csghub-lite_*_linux_amd64.tar.gz` |
| Linux ARM64 | `csghub-lite_*_linux_arm64.tar.gz` |
| Windows x86_64 | `csghub-lite_*_windows_amd64.zip` |

下载后解压并移动到 PATH 中：

```bash
tar xzf csghub-lite_*.tar.gz
sudo mv csghub-lite /usr/local/bin/
```

### 方式四：Linux 包管理器

Debian / Ubuntu：

```bash
sudo dpkg -i csghub-lite_*.deb
```

RHEL / CentOS / Fedora：

```bash
sudo rpm -i csghub-lite_*.rpm
```

### 方式五：从源码编译

```bash
git clone https://github.com/opencsgs/csghub-lite.git
cd csghub-lite
make build
# 二进制文件位于 bin/csghub-lite
```

全平台编译：

```bash
make build-all
```

## 安装 llama-server（推理依赖）

csghub-lite 使用 llama.cpp 的 `llama-server` 进行模型推理。你需要单独安装它。

### macOS

```bash
brew install llama.cpp
```

### Linux / Windows

从 [llama.cpp Releases](https://github.com/ggml-org/llama.cpp/releases) 下载对应平台的预编译包，解压后将 `llama-server` 放入 PATH 即可。

```bash
# 示例：Linux x86_64
wget https://github.com/ggml-org/llama.cpp/releases/download/b8429/llama-b8429-bin-ubuntu-x64.tar.gz
tar xzf llama-b8429-bin-ubuntu-x64.tar.gz
sudo cp build/bin/llama-server /usr/local/bin/
```

## 验证安装

```bash
csghub-lite --version
llama-server --version
```
