# csghub-lite chat

与已下载的本地模型启动交互式对话。

## 用法

```bash
csghub-lite chat <model> [flags]
```

## 参数

| 参数 | 说明 |
|------|------|
| `<model>` | 模型名称，格式 `namespace/name` |

## 选项

| 选项 | 说明 |
|------|------|
| `--system <prompt>` | 设置自定义系统提示词 |

## 与 run 的区别

| | `run` | `chat` |
|---|---|---|
| 自动下载模型 | 是 | 否 |
| 自定义系统提示词 | 不支持 | `--system` 选项 |
| 适用场景 | 首次使用 | 模型已下载后的日常使用 |

## 交互命令

| 命令 | 说明 |
|------|------|
| `/bye`、`/exit`、`/quit` | 退出对话 |
| `/clear` | 清除上下文，重新开始 |
| `/help` | 显示帮助 |

## 示例

```bash
# 基本用法
csghub-lite chat Qwen/Qwen3-0.6B-GGUF

# 设置自定义系统提示词
csghub-lite chat Qwen/Qwen3-0.6B-GGUF --system "你是一个编程助手，只用中文回答。"

# 设置为翻译助手
csghub-lite chat Qwen/Qwen3-0.6B-GGUF --system "You are a translator. Translate all input to English."
```

## 错误处理

如果模型未下载，会提示先使用 `pull` 命令：

```
Error: model "Qwen/Qwen3-0.6B-GGUF" not found locally. Use 'csghub-lite pull Qwen/Qwen3-0.6B-GGUF' to download it first
```
