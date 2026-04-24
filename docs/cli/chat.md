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
| `--num-ctx <n>` | 仅对本次 `chat` 生效的上下文长度 |
| `--num-parallel <n>` | 仅对本次 `chat` 生效的并行槽数；设为 `1` 可优先给单会话更大上下文 |
| `--n-gpu-layers <n>` | 仅对本次 `chat` 生效的 llama-server `--n-gpu-layers`；可用 `0` 禁用 GPU offload，或指定层数例如 `40` |
| `--cache-type-k <type>` | 仅对本次 `chat` 生效的 llama-server `--cache-type-k`，可用值：`f32`、`f16`、`bf16`、`q8_0`、`q4_0`、`q4_1`、`iq4_nl`、`q5_0`、`q5_1` |
| `--cache-type-v <type>` | 仅对本次 `chat` 生效的 llama-server `--cache-type-v`，可用值：`f32`、`f16`、`bf16`、`q8_0`、`q4_0`、`q4_1`、`iq4_nl`、`q5_0`、`q5_1` |
| `--dtype <type>` | 仅对本次 `chat` 生效的 SafeTensors -> GGUF 转换输出类型，可用值：`f32`、`f16`、`bf16`、`q8_0`、`tq1_0`、`tq2_0`、`auto` |

## 与 run 的区别

| | `run` | `chat` |
|---|---|---|
| 自动下载模型 | 是 | 否 |
| 自定义系统提示词 | 不支持 | `--system` 选项 |
| 适用场景 | 首次使用 | 模型已下载后的日常使用 |

`--dtype` 只在模型需要从 SafeTensors 自动转换为 GGUF 时生效；如果是视觉模型，匹配的 `mmproj` 也会按同一 `dtype` 一起转换。若模型目录里已经有对应 `dtype` 的 GGUF / `mmproj` 文件，则会直接复用。

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

# 显存紧张时，压缩 KV cache dtype
csghub-lite chat Qwen/Qwen3-0.6B-GGUF --cache-type-k q8_0 --cache-type-v q8_0

# 控制 GPU offload 层数
csghub-lite chat Qwen/Qwen3-0.6B-GGUF --n-gpu-layers 40

# 首次自动转换 SafeTensors 时，直接生成 BF16 GGUF
csghub-lite chat Qwen/Qwen3-0.6B --dtype bf16

# 设置为翻译助手
csghub-lite chat Qwen/Qwen3-0.6B-GGUF --system "You are a translator. Translate all input to English."
```

## 错误处理

如果模型未下载，会提示先使用 `pull` 命令：

```
Error: model "Qwen/Qwen3-0.6B-GGUF" not found locally. Use 'csghub-lite pull Qwen/Qwen3-0.6B-GGUF' to download it first
```
