# csghub-lite search

在 CSGHub 平台上搜索模型。

## 用法

```bash
csghub-lite search <query>
```

## 参数

| 参数 | 说明 |
|------|------|
| `<query>` | 搜索关键词 |

## 输出字段

| 字段 | 说明 |
|------|------|
| NAME | 模型名称（`namespace/name`） |
| DOWNLOADS | 下载次数 |
| LICENSE | 许可证 |
| DESCRIPTION | 模型描述（截断显示） |

## 示例

```bash
# 搜索 Qwen 系列模型
csghub-lite search "Qwen3"

# 搜索 GGUF 格式的模型（可直接推理）
csghub-lite search "Qwen3 GGUF"

# 搜索中文模型
csghub-lite search "中文对话"
```

## 提示

- 推理需要 GGUF 格式的模型，搜索时加上 "GGUF" 关键词
- SafeTensors 格式的模型需要先转换为 GGUF 才能推理
