# 模型管理与服务管理 API

## <a id="health"></a>GET /api/health

健康检查。

**响应:**

```json
{"status": "ok"}
```

---

## <a id="list"></a>GET /api/tags

列出所有已下载的本地模型。

**响应:**

```json
{
  "models": [
    {
      "name": "Qwen/Qwen3-0.6B-GGUF",
      "model": "Qwen/Qwen3-0.6B-GGUF",
      "size": 639466546,
      "format": "gguf",
      "modified_at": "2026-03-11T00:42:14.856Z"
    }
  ]
}
```

---

## <a id="show"></a>POST /api/show

查看模型详细信息。

**请求:**

```json
{"model": "Qwen/Qwen3-0.6B-GGUF"}
```

**响应:**

```json
{
  "modelfile": "",
  "details": {
    "name": "Qwen/Qwen3-0.6B-GGUF",
    "model": "Qwen/Qwen3-0.6B-GGUF",
    "size": 639466546,
    "format": "gguf",
    "modified_at": "2026-03-11T00:42:14.856Z"
  }
}
```

---

## <a id="pull"></a>POST /api/pull

从 CSGHub 下载模型。返回 SSE 流式进度。

**请求:**

```json
{"model": "Qwen/Qwen3-0.6B-GGUF"}
```

**响应（SSE）:**

```
data: {"status":"pulling Qwen/Qwen3-0.6B-GGUF"}

data: {"status":"downloading Qwen3-0.6B-Q8_0.gguf","digest":"Qwen3-0.6B-Q8_0.gguf","total":639446688,"completed":32768}

data: {"status":"success"}
```

| 字段 | 说明 |
|------|------|
| `status` | 状态描述 |
| `digest` | 当前下载的文件名 |
| `total` | 文件总字节数 |
| `completed` | 已下载字节数 |

---

## <a id="delete"></a>DELETE /api/delete

删除本地模型。

**请求:**

```json
{"model": "Qwen/Qwen3-0.6B-GGUF"}
```

**响应:**

```json
{"status": "deleted"}
```

---

## <a id="ps"></a>GET /api/ps

列出服务器上当前加载并运行的模型。

**响应:**

```json
{
  "models": [
    {
      "name": "Qwen/Qwen3-0.6B-GGUF",
      "model": "Qwen/Qwen3-0.6B-GGUF",
      "size": 639466546,
      "format": "gguf",
      "expires_at": "0001-01-01T00:00:00Z"
    }
  ]
}
```

| 字段 | 说明 |
|------|------|
| `name` | 模型名称 |
| `size` | 模型大小（字节） |
| `format` | 模型格式 |
| `expires_at` | 到期卸载时间（零值表示永久） |

---

## <a id="stop"></a>POST /api/stop

停止并卸载一个运行中的模型，释放内存和 GPU 资源。

**请求:**

```json
{"model": "Qwen/Qwen3-0.6B-GGUF"}
```

**响应:**

```json
{"status": "stopped"}
```

**错误（模型未运行）:**

```json
{"error": "model \"Qwen/Qwen3-0.6B-GGUF\" is not running"}
```
