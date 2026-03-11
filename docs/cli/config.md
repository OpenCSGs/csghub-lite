# csghub-lite config

查看和管理 csghub-lite 的配置。

## 子命令

### config show

显示当前所有配置：

```bash
csghub-lite config show
```

### config get

获取某个配置项的值：

```bash
csghub-lite config get <key>
```

### config set

设置某个配置项：

```bash
csghub-lite config set <key> <value>
```

## 配置项

| Key | 默认值 | 说明 |
|-----|--------|------|
| `server_url` | `https://hub.opencsg.com` | CSGHub 平台地址 |
| `model_dir` | `~/.csghub-lite/models` | 本地模型存储目录 |
| `listen_addr` | `:11435` | API 服务监听地址 |
| `token` | （空） | CSGHub 访问令牌 |

## 配置文件

配置文件位于 `~/.csghub-lite/config.json`，JSON 格式：

```json
{
  "server_url": "https://hub.opencsg.com",
  "listen_addr": ":11435",
  "model_dir": "/Users/user/.csghub-lite/models"
}
```

## 示例

```bash
# 查看所有配置
csghub-lite config show

# 切换到私有化部署
csghub-lite config set server_url https://my-csghub.example.com

# 修改模型存储路径
csghub-lite config set model_dir /data/models

# 修改 API 端口
csghub-lite config set listen_addr :8080

# 查看某个配置值
csghub-lite config get server_url
```
