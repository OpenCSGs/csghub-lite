# 打包与发布

csghub-lite 使用 [GoReleaser](https://goreleaser.com) 进行跨平台构建和发布。

## 支持的分发形式

| 分发方式 | 平台 | 文件/命令 |
|---------|------|-----------|
| 一键安装脚本 | Linux / macOS | `curl -fsSL .../install.sh \| sh` |
| Homebrew | macOS / Linux | `brew install csghub-lite` |
| tar.gz | macOS / Linux | GitHub Releases |
| zip | Windows | GitHub Releases |
| deb | Debian / Ubuntu | GitHub Releases |
| rpm | RHEL / CentOS / Fedora | GitHub Releases |
| 源码编译 | 全平台 | `make build` |

## 构建

### 本地构建

```bash
# 构建当前平台
make build

# 构建全平台
make build-all

# 二进制文件输出到 bin/ 目录
ls bin/
csghub-lite                   # 当前平台
csghub-lite-darwin-arm64      # macOS Apple Silicon
csghub-lite-darwin-amd64      # macOS Intel
csghub-lite-linux-amd64       # Linux x86_64
csghub-lite-linux-arm64       # Linux ARM64
csghub-lite-windows-amd64.exe # Windows
```

### 版本号

版本号通过 `git tag` 管理，自动嵌入二进制文件：

```bash
git tag v0.1.0
git push origin v0.1.0
```

本地开发时版本号为 `dev`。

## GoReleaser 配置

配置文件: `.goreleaser.yml`

### 核心设置

```yaml
builds:
  - id: csghub-lite
    main: ./cmd/csghub-lite
    env:
      - CGO_ENABLED=0    # 纯 Go 编译，无 CGO 依赖
    goos: [linux, darwin, windows]
    goarch: [amd64, arm64]
    ldflags:
      - -s -w -X main.version={{.Version}}
```

### 包格式

- **tar.gz**: Linux / macOS 默认
- **zip**: Windows 默认
- **deb / rpm**: 通过 NFPM 生成

### Homebrew Tap

GoReleaser 自动将 Homebrew formula 推送到 `opencsgs/homebrew-tap` 仓库。需要配置 `HOMEBREW_TAP_TOKEN` secret。

## CI/CD

通过 GitHub Actions 自动化，配置文件: `.github/workflows/ci.yml`

### 触发条件

- **Push / PR 到 main**: 运行测试
- **Push tag `v*`**: 运行测试 + GoReleaser 发布

### 发布流程

```bash
# 1. 确保测试通过
make test

# 2. 创建版本标签
git tag v0.1.0

# 3. 推送标签（触发 CI 自动发布）
git push origin v0.1.0
```

CI 自动执行：

1. 运行所有测试
2. GoReleaser 交叉编译所有平台
3. 上传到 GitHub Releases
4. 更新 Homebrew tap
5. 生成 deb / rpm 包

### GitLab 同步（CSGHub / 国内安装源）

推送 `v*` 标签只会触发 **GitHub** 上的 GoReleaser 发布；**GitLab** 上的 Release 与 Generic Package 需要单独上传，否则 `install.sh` 在国内（CN）从 GitLab 拉取时会缺包。

**推荐（自动化）**：在 GitHub 仓库 **Settings → Secrets and variables → Actions** 中新增 `GITLAB_TOKEN`（GitLab 个人访问令牌，`api` 权限）。发布工作流在 GoReleaser 成功后会调用 `scripts/push.sh`，将五个平台压缩包同步到 `git-devops.opencsg.com` 上与本项目 `scripts/push.sh` 一致的 Generic Package 路径。未配置该 secret 时此步骤跳过，行为与以前一致。

**手动补发某个版本**（例如已发 GitHub 但未发 GitLab）：

```bash
# 从 GitHub Release 拉取 GoReleaser 产物到 dist/
gh release download v0.5.10 --repo OpenCSGs/csghub-lite -D dist/

# 文件名改为与 install.sh / push.sh 一致（OS-ARCH 用连字符）
./scripts/rename-dist-for-gitlab.sh 0.5.10

export GITLAB_TOKEN="glpat-..."   # 或写入 local/secrets.env 后由 push.sh 自动加载
./scripts/push.sh --skip-github --skip-build --skip-gitlab-git --tag v0.5.10
```

本地从零构建再上传时，检出对应 tag 后执行 `make package`，然后 `scripts/push.sh --skip-github --skip-build --tag vX.Y.Z`（会包含向 GitLab 推送 tag，除非加 `--skip-gitlab-git`）。

### 本地测试发布

```bash
# GoReleaser 本地快照（不实际发布）
make release-snapshot

# 输出到 dist/ 目录
ls dist/
```

## 安装脚本

`scripts/install.sh` 提供一键安装：

- 自动检测 OS 和 CPU 架构
- 从 GitHub Releases 下载最新版本
- 解压并安装到 `/usr/local/bin`
- 检查 `llama-server` 是否可用

支持指定版本：

```bash
CSGHUB_LITE_VERSION=v0.1.0 curl -fsSL .../install.sh | sh
```

## Homebrew Formula

独立的 Homebrew formula 位于 `Formula/csghub-lite.rb`，主要用于本地测试。正式发布由 GoReleaser 自动推送到 `opencsgs/homebrew-tap`。
