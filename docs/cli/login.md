# csghub-lite login

设置 CSGHub 平台的访问令牌（Access Token）。

## 用法

```bash
csghub-lite login
```

## 说明

运行后会提示输入 Access Token（输入内容不会回显）。Token 保存在配置文件 `~/.csghub-lite/config.json` 中。

## 获取 Token

1. 访问 [hub.opencsg.com](https://hub.opencsg.com)（或你的私有化部署地址）
2. 登录账户
3. 进入 **设置** > **Access Token**
4. 创建或复制 Token

## 何时需要登录

- 访问私有模型时必须登录
- 公开模型不需要登录即可下载和使用

## 示例

```bash
$ csghub-lite login
Enter your CSGHub access token: ********
Token saved successfully.
```

## 相关命令

- [`config set token <value>`](config.md) — 也可以通过 config 命令设置 token
