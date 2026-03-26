# 打包与发布

csghub-lite 当前以本地 `make package` + `scripts/push.sh` 手动发布为主。GoReleaser 仍然保留用于定义归档格式、生成 GitHub Release 产物，以及执行本地 snapshot 验证，但不再负责 Homebrew tap 发布。

## 支持的分发形式

| 分发方式 | 平台 | 文件/命令 |
|---------|------|-----------|
| 一键安装脚本 | Linux / macOS | `curl -fsSL https://hub.opencsg.com/csghub-lite/install.sh \| sh` |
| Homebrew | macOS | `brew tap opencsgs/csghub-lite https://github.com/OpenCSGs/csghub-lite && brew install opencsgs/csghub-lite/csghub-lite` |
| tar.gz | macOS / Linux | GitHub Releases |
| zip | Windows | GitHub Releases |
| deb | Debian / Ubuntu | GitHub Releases |
| rpm | RHEL / CentOS / Fedora | GitHub Releases |
| 源码编译 | 全平台 | `make build` |

## 本地构建

```bash
# 构建当前平台
make build

# 构建全平台
make build-all

# 打包发布产物
make package
```

`make build`、`make build-all` 与 `make package` 都会先构建 `web` 并同步到 `internal/server/static`，确保发布二进制内嵌 Web UI。`make package` 还会额外生成 `dist/checksums.txt`，供 Homebrew formula 和发布校验复用。

## 版本号

版本号通过 `git tag` 管理，并在构建时注入二进制：

```bash
git tag v0.1.0
git push origin v0.1.0
```

未打 tag 的本地开发构建默认显示为 `dev`。

## 推荐发布流程

```bash
# 1. 确保测试通过
make test

# 2. 创建发布 tag
git tag v0.1.0

# 3. 本地打包（会构建 web 并生成 dist/checksums.txt）
make package

# 4. 更新仓库内 Homebrew formula
./scripts/update-homebrew-formula.sh --tag v0.1.0

# 5. 上传 GitHub / GitLab release 资产
./scripts/push.sh --skip-build --tag v0.1.0
```

说明：

- `scripts/push.sh` 会将本地 `dist/` 下的发布包上传到 GitHub Release 和 GitLab Generic Package/Release。
- GitLab 上传会自动从 `local/secrets.env` 读取 `GITLAB_TOKEN`（如果环境变量未设置）。
- 如果你希望仓库中的 `Formula/csghub-lite.rb` 始终指向“最新正式版”，请在发布完成后提交该文件的更新。

## GitLab 补发

如果某个版本已经发到了 GitHub，但 GitLab 资产缺失，可以先把 release 文件拉回本地再补发：

```bash
gh release download v0.5.10 --repo OpenCSGs/csghub-lite -D dist/
./scripts/rename-dist-for-gitlab.sh 0.5.10
./scripts/push.sh --skip-github --skip-build --skip-gitlab-git --tag v0.5.10
```

## 安装脚本

主安装入口保持不变：

```bash
curl -fsSL https://hub.opencsg.com/csghub-lite/install.sh | sh
```

`scripts/install.sh` 会自动检测 OS 和 CPU 架构，优先从 GitHub/GitLab release 资产下载最新版本，并在需要时安装或升级 `llama-server`。

macOS 上，安装脚本会优先选择当前 `PATH` 中可写的目录（例如 `/opt/homebrew/bin`）；如果没有合适的目录，则回退到 `~/bin`，并自动写入 shell 配置，尽量避免 `sudo`。

指定版本安装：

```bash
CSGHUB_LITE_VERSION=v0.1.0 curl -fsSL https://hub.opencsg.com/csghub-lite/install.sh | sh
```

## Homebrew Formula

Homebrew 现在是 repo 内维护的额外入口，主要面向 macOS。主仓库本身充当自定义 tap，不依赖独立的 `homebrew-tap` 仓库，也不需要向 Homebrew 上传二进制文件。

Linux 的正式安装文档仍以 `install.sh`、release 压缩包和 `deb/rpm` 为主。

- Formula 文件位于 `Formula/csghub-lite.rb`
- 更新脚本位于 `scripts/update-homebrew-formula.sh`
- 该脚本读取 `dist/checksums.txt`，将当前 release 的 URL 和 SHA256 写回 formula

用户先把主仓库 tap 进 Homebrew，再安装对应 formula：

```bash
brew tap opencsgs/csghub-lite https://github.com/OpenCSGs/csghub-lite
brew install opencsgs/csghub-lite/csghub-lite
```

## GoReleaser 与 CI

- `.goreleaser.yml` 继续定义 archive、checksum、nfpm 和 GitHub release 相关配置
- `make release-snapshot` 可在本地验证 GoReleaser 输出
- GitHub Actions 仍会在 tag 上构建 release 产物，但仓库约定的正式发布方式仍然是本地打包后手动上传
