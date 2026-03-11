# csghub-lite list

列出所有已下载到本地的模型。

## 用法

```bash
csghub-lite list
csghub-lite ls
```

## 别名

`ls` 是 `list` 的别名。

## 输出字段

| 字段 | 说明 |
|------|------|
| NAME | 模型名称（`namespace/name`） |
| FORMAT | 模型格式（gguf / safetensors / unknown） |
| SIZE | 模型总大小 |
| DOWNLOADED | 下载时间 |

## 示例

```bash
$ csghub-lite list
NAME                   FORMAT   SIZE       DOWNLOADED
Qwen/Qwen3-0.6B-GGUF   gguf     609.8 MB   2026-03-11 00:42
```

没有模型时：

```bash
$ csghub-lite list
No models downloaded. Use 'csghub-lite pull' to download a model.
```
