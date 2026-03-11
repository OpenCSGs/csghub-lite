# csghub-lite ps

列出服务器上当前正在运行的模型。

## 用法

```bash
csghub-lite ps
```

## 前提条件

需要先启动服务器：`csghub-lite serve`

## 输出字段

| 字段 | 说明 |
|------|------|
| NAME | 模型名称 |
| FORMAT | 模型格式 |
| SIZE | 模型大小 |
| UNTIL | 剩余保活时间 |

## 示例

```bash
$ csghub-lite ps
NAME                   FORMAT   SIZE       UNTIL
Qwen/Qwen3-0.6B-GGUF   gguf     609.8 MB   forever
```

没有运行中的模型时：

```bash
$ csghub-lite ps
No models currently running.
```

服务器未启动时：

```
Error: cannot connect to csghub-lite server at http://localhost:11435. Is it running? Start it with 'csghub-lite serve'
```
