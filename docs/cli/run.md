# csghub-lite run

自动下载模型（如果本地不存在）并启动交互式对话。

## 用法

```bash
csghub-lite run <model> [flags]
```

## 参数

| 参数 | 说明 |
|------|------|
| `<model>` | 模型名称，格式 `namespace/name` |

## 选项

| 选项 | 说明 | 默认值 |
|------|------|--------|
| `--num-ctx <n>` | 仅对本次 `run` 生效的上下文长度 | 使用服务默认值 |
| `--num-parallel <n>` | 仅对本次 `run` 生效的并行槽数；设为 `1` 可优先给单会话更大上下文 | 使用服务默认值 |
| `--n-gpu-layers <n>` | 仅对本次 `run` 生效的 llama-server `--n-gpu-layers`；可用 `0` 禁用 GPU offload，或指定层数例如 `40` | GPU 环境下默认尽量全量 offload，CPU 环境下不设置 |
| `--cache-type-k <type>` | 仅对本次 `run` 生效的 llama-server `--cache-type-k`，可用值：`f32`、`f16`、`bf16`、`q8_0`、`q4_0`、`q4_1`、`iq4_nl`、`q5_0`、`q5_1` | llama-server 默认值 |
| `--cache-type-v <type>` | 仅对本次 `run` 生效的 llama-server `--cache-type-v`，可用值：`f32`、`f16`、`bf16`、`q8_0`、`q4_0`、`q4_1`、`iq4_nl`、`q5_0`、`q5_1` | llama-server 默认值 |
| `--dtype <type>` | 仅对本次 `run` 生效的 SafeTensors -> GGUF 转换输出类型，可用值：`f32`、`f16`、`bf16`、`q8_0`、`tq1_0`、`tq2_0`、`auto` | `f16` |

## 说明

`run` 是最常用的命令，适合首次使用。它会：

1. 检查模型是否已下载到本地
2. 如未下载，自动从 CSGHub 拉取
3. 加载模型并启动交互对话

`--dtype` 只在模型需要从 SafeTensors 自动转换为 GGUF 时生效；如果是视觉模型，匹配的 `mmproj` 也会按同一 `dtype` 一起转换。若模型目录里已经有对应 `dtype` 的 GGUF / `mmproj` 文件，则会直接复用。

如果只想对话而不自动下载，使用 [`chat`](chat.md) 命令。

## 交互命令

进入对话后，可使用以下命令：

| 命令 | 说明 |
|------|------|
| `/bye`、`/exit`、`/quit` | 退出对话 |
| `/clear` | 清除上下文，重新开始 |
| `/help` | 显示帮助 |

- 行尾输入 `\` 可以换行输入（多行模式）
- 按 `Ctrl+D` 退出

## 示例

```bash
# 运行 Qwen3-0.6B 模型
csghub-lite run Qwen/Qwen3-0.6B-GGUF

# 为单次运行显式指定更大的上下文
csghub-lite run Qwen/Qwen3-0.6B-GGUF --num-ctx 131072 --num-parallel 1

# 控制 GPU offload 层数
csghub-lite run Qwen/Qwen3-0.6B-GGUF --n-gpu-layers 40

# 显存紧张时，压缩 KV cache dtype
csghub-lite run Qwen/Qwen3-0.6B-GGUF --cache-type-k q8_0 --cache-type-v q8_0

# 首次自动转换 SafeTensors 时，直接生成 Q8_0 GGUF
csghub-lite run Qwen/Qwen3-0.6B --dtype q8_0

# 运行后进入交互模式
>>> 用一句话介绍你自己
我是AI助手，专注于帮助用户解决问题和获取信息。

>>> 1+1等于多少
1 + 1 等于 2。

>>> /bye
Goodbye!
```
