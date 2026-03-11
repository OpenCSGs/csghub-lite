# 配置说明

## 配置文件

配置文件位于 `~/.csghub-lite/config.json`，首次运行时自动创建。

```json
{
  "server_url": "https://hub.opencsg.com",
  "token": "",
  "listen_addr": ":11435",
  "model_dir": "/Users/user/.csghub-lite/models"
}
```

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

### model_dir

本地模型的存储目录。

- 默认值: `~/.csghub-lite/models`
- 目录结构: `<model_dir>/<namespace>/<name>/`

```bash
# 使用大容量磁盘
csghub-lite config set model_dir /data/llm-models
```

## 管理命令

```bash
# 查看所有配置
csghub-lite config show

# 获取单个配置
csghub-lite config get server_url

# 设置配置
csghub-lite config set server_url https://my-csghub.example.com
```

## 目录结构

```
~/.csghub-lite/
├── config.json                    # 配置文件
└── models/                        # 模型存储目录
    └── Qwen/
        └── Qwen3-0.6B-GGUF/
            ├── manifest.json      # 模型元信息
            ├── Qwen3-0.6B-Q8_0.gguf
            ├── README.md
            ├── LICENSE
            └── ...
```
