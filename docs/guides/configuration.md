# 配置说明

## 配置文件

配置文件位于 `~/.csghub-lite/config.json`，首次运行时自动创建。

```json
{
  "server_url": "https://hub.opencsg.com",
  "token": "",
  "listen_addr": ":11435",
  "model_dir": "/Users/user/.csghub-lite/models",
  "dataset_dir": "/Users/user/.csghub-lite/datasets"
}
```

说明：CLI 和 Web 设置页提供 `storage_dir` 这个便捷配置项。设置它时，csghub-lite 会自动把模型和数据集目录展开为 `model_dir` 与 `dataset_dir`。

## 配置项详解

### server_url

CSGHub 平台的 API 地址。

- 默认值: `https://hub.opencsg.com`
- 用途: 模型搜索、信息查询、文件下载的 API 基地址

切换到私有化部署：

```bash
csghub-lite config set server_url https://my-csghub.example.com
```

### token

CSGHub 平台的访问令牌（Access Token）。

- 默认值: 空
- 用途: 访问私有模型时的身份认证

设置方式：

```bash
# 交互式输入（推荐，不会留在 shell 历史中）
csghub-lite login

# 或通过 config 命令
csghub-lite config set token your-token-here
```

### listen_addr

REST API 服务的监听地址。

- 默认值: `:11435`
- 格式: `[host]:port`

```bash
# 修改端口
csghub-lite config set listen_addr :8080

# 仅监听本地
csghub-lite config set listen_addr 127.0.0.1:11435

# 也可以在启动时临时指定
csghub-lite serve --listen :9090
```

### storage_dir

模型和数据集共用的本地存储根目录。

- 默认值: `~/.csghub-lite`
- 目录结构:
  - 模型: `<storage_dir>/models/<namespace>/<name>/`
  - 数据集: `<storage_dir>/datasets/<namespace>/<name>/`

```bash
# 使用大容量磁盘
csghub-lite config set storage_dir /data/csghub-lite
```

### model_dir / dataset_dir

这两个是实际生效的模型目录和数据集目录，通常由 `storage_dir` 自动派生：

- `model_dir = <storage_dir>/models`
- `dataset_dir = <storage_dir>/datasets`

如果确实需要，也可以单独覆盖：

```bash
csghub-lite config set model_dir /data/models
csghub-lite config set dataset_dir /data/datasets
```

## 管理命令

```bash
# 查看所有配置
csghub-lite config show

# 获取单个配置
csghub-lite config get server_url
csghub-lite config get storage_dir

# 设置配置
csghub-lite config set server_url https://my-csghub.example.com
```

## 目录结构

```
~/.csghub-lite/
├── config.json                    # 配置文件
├── models/                        # 模型存储目录
│   └── Qwen/
│       └── Qwen3-0.6B-GGUF/
│           ├── manifest.json      # 模型元信息
│           ├── Qwen3-0.6B-Q8_0.gguf
│           ├── README.md
│           ├── LICENSE
│           └── ...
└── datasets/                      # 数据集存储目录
```
