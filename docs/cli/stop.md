# csghub-lite stop

停止并卸载服务器上正在运行的模型，释放内存和 GPU 资源。

## 用法

```bash
csghub-lite stop <model>
```

## 参数

| 参数 | 说明 |
|------|------|
| `<model>` | 模型名称，格式 `namespace/name` |

## 前提条件

需要先启动服务器：`csghub-lite serve`

## 说明

- 停止模型会终止底层的 `llama-server` 进程
- 停止后模型不再占用内存
- 下次请求该模型时会重新加载

## 示例

```bash
# 停止指定模型
csghub-lite stop Qwen/Qwen3-0.6B-GGUF
Stopped model Qwen/Qwen3-0.6B-GGUF
```
