# csghub-lite show

显示已下载模型的详细信息。

## 用法

```bash
csghub-lite show <model>
```

## 参数

| 参数 | 说明 |
|------|------|
| `<model>` | 模型名称，格式 `namespace/name` |

## 输出字段

| 字段 | 说明 |
|------|------|
| Name | 模型全名 |
| Format | 模型格式 |
| Size | 总大小 |
| Downloaded | 下载时间 |
| License | 许可证 |
| Description | 模型描述 |
| Path | 本地存储路径 |
| Files | 文件列表 |

## 示例

```bash
$ csghub-lite show Qwen/Qwen3-0.6B-GGUF
Name:         Qwen/Qwen3-0.6B-GGUF
Format:       gguf
Size:         609.8 MB
Downloaded:   2026-03-11 00:42:14
License:      apache-2.0
Description:  Qwen3-0.6B-GGUF是Qwen系列最新的大语言模型...
Path:         /Users/user/.csghub-lite/models/Qwen/Qwen3-0.6B-GGUF
Files:        .gitattributes, LICENSE, Qwen3-0.6B-Q8_0.gguf, README.md, params
```
