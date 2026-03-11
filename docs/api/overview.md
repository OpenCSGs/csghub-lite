# REST API 总览

csghub-lite 提供 Ollama 兼容的 REST API，通过 `csghub-lite serve` 启动。

## 基本信息

- 默认地址: `http://localhost:11435`
- 内容类型: `application/json`
- 流式响应: `text/event-stream`（SSE 格式）

## 端点列表

### 推理

| 方法 | 路径 | 说明 | 文档 |
|------|------|------|------|
| `POST` | `/api/chat` | 对话补全（流式/非流式） | [详情](chat.md) |
| `POST` | `/api/generate` | 文本生成（流式/非流式） | [详情](generate.md) |

### 模型管理

| 方法 | 路径 | 说明 | 文档 |
|------|------|------|------|
| `GET` | `/api/tags` | 列出本地模型 | [详情](models.md#list) |
| `POST` | `/api/show` | 查看模型详情 | [详情](models.md#show) |
| `POST` | `/api/pull` | 下载模型（流式进度） | [详情](models.md#pull) |
| `DELETE` | `/api/delete` | 删除模型 | [详情](models.md#delete) |

### 服务管理

| 方法 | 路径 | 说明 | 文档 |
|------|------|------|------|
| `GET` | `/api/health` | 健康检查 | [详情](models.md#health) |
| `GET` | `/api/ps` | 列出运行中的模型 | [详情](models.md#ps) |
| `POST` | `/api/stop` | 停止运行中的模型 | [详情](models.md#stop) |

## 流式响应

默认情况下，`/api/chat` 和 `/api/generate` 使用 SSE（Server-Sent Events）流式返回。每个事件格式为：

```
data: {"model":"...","message":{"role":"assistant","content":"token"},"done":false}

data: {"model":"...","done":true}
```

设置 `"stream": false` 可获取完整的非流式 JSON 响应。

## 通用选项

推理请求支持以下生成参数：

```json
{
  "options": {
    "temperature": 0.7,
    "top_p": 0.9,
    "top_k": 40,
    "max_tokens": 2048,
    "seed": -1,
    "num_ctx": 4096
  }
}
```

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `temperature` | 0.7 | 温度，越高越随机 |
| `top_p` | 0.9 | 核采样概率 |
| `top_k` | 40 | Top-K 采样 |
| `max_tokens` | 2048 | 最大生成 token 数 |
| `seed` | -1 | 随机种子（-1 为随机） |
| `num_ctx` | 4096 | 上下文窗口大小 |
