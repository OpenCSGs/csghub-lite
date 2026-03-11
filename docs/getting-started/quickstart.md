# 快速入门

本指南帮助你在 5 分钟内完成：搜索模型、下载模型、交互对话、启动 API 服务。

## 前提条件

确保已安装 `csghub-lite` 和 `llama-server`。详见 [安装指南](installation.md)。

## 第一步：登录（可选）

如果需要访问私有模型，设置 CSGHub 访问令牌：

```bash
csghub-lite login
```

按提示输入你在 [hub.opencsg.com](https://hub.opencsg.com) 上的 Access Token。

公开模型不需要登录。

## 第二步：搜索模型

```bash
csghub-lite search "Qwen3"
```

输出示例：

```
Found 34 models (showing top 34):

NAME                                DOWNLOADS   LICENSE      DESCRIPTION
Qwen/Qwen3-0.6B-GGUF               40          apache-2.0   Qwen3-0.6B-GGUF...
Qwen/Qwen3-0.6B                    289229      apache-2.0   ...
...
```

> 提示：推理需要 GGUF 格式的模型，搜索时可加上 "GGUF" 关键词。

## 第三步：运行模型

使用 `run` 命令，自动下载并进入交互对话：

```bash
csghub-lite run Qwen/Qwen3-0.6B-GGUF
```

首次运行会自动下载模型（约 610MB），然后进入交互模式：

```
Loading Qwen/Qwen3-0.6B-GGUF...
Model Qwen/Qwen3-0.6B-GGUF ready. Type '/bye' to exit, '/clear' to reset context.

>>> 你好，介绍一下自己
我是AI助手，专注于帮助用户解决问题和获取信息。

>>> /bye
Goodbye!
```

## 第四步：管理模型

```bash
# 查看已下载模型
csghub-lite list

# 查看模型详情
csghub-lite show Qwen/Qwen3-0.6B-GGUF

# 删除模型
csghub-lite rm Qwen/Qwen3-0.6B-GGUF
```

## 第五步：启动 API 服务

```bash
csghub-lite serve
```

服务默认监听 `localhost:11435`，可以用 curl 测试：

```bash
# 健康检查
curl http://localhost:11435/api/health

# 对话
curl http://localhost:11435/api/chat -d '{
  "model": "Qwen/Qwen3-0.6B-GGUF",
  "messages": [{"role": "user", "content": "Hello!"}]
}'

# 查看运行中的模型
curl http://localhost:11435/api/ps
```

## 下一步

- [CLI 命令参考](../cli/overview.md) — 查看所有命令的详细用法
- [REST API 参考](../api/overview.md) — 了解完整的 API 接口
- [配置说明](../guides/configuration.md) — 切换私有化 CSGHub 部署
