# 安装指南

## 系统要求

- **操作系统**: macOS (Apple Silicon / Intel)、Linux (x86_64 / ARM64)、Windows (x86_64)
- **推理依赖**: [llama-server](https://github.com/ggml-org/llama.cpp)（模型推理必需）
- **编译依赖**: Go 1.22+（仅源码编译需要）

## 安装 csghub-lite

### 方式一：一键安装脚本（推荐）

适用于 Linux 和 macOS，自动检测系统架构，从 GitHub Releases 下载安装。

```bash
curl -fsSL https://raw.githubusercontent.com/opencsgs/csghub-lite/main/scripts/install.sh | sh
```

指定版本安装：

```bash
CSGHUB_LITE_VERSION=v0.1.0 curl -fsSL https://raw.githubusercontent.com/opencsgs/csghub-lite/main/scripts/install.sh | sh
```

### 方式二：Homebrew

适用于 macOS 和 Linux。

```bash
brew tap opencsgs/tap
brew install csghub-lite
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
