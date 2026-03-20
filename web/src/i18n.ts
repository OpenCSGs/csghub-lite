import { signal } from "@preact/signals";

export type Locale = "en" | "zh";

function detectLocale(): Locale {
  const lang = navigator.language || "";
  return lang.startsWith("zh") ? "zh" : "en";
}

export const locale = signal<Locale>(detectLocale());

export function setLocale(l: Locale) {
  locale.value = l;
  localStorage.setItem("csghub-locale", l);
}

const saved = localStorage.getItem("csghub-locale");
if (saved === "en" || saved === "zh") locale.value = saved;

const en: Record<string, string> = {
  // Nav
  "nav.dashboard": "Dashboard",
  "nav.marketplace": "Marketplace",
  "nav.library": "Library",
  "nav.chat": "Chat",

  // Dashboard
  "dash.resource": "RESOURCE UTILIZATION",
  "dash.updates": "Updates every 3s",
  "dash.cpu": "CPU USAGE",
  "dash.ram": "RAM USAGE",
  "dash.gpu": "GPU VRAM",
  "dash.na": "N/A",
  "dash.active": "ACTIVE INFERENCE MODELS",
  "dash.noModels": "No models currently running.",
  "dash.modelName": "MODEL NAME",
  "dash.expiresAt": "Expires At",
  "dash.actions": "Actions",
  "dash.format": "Format",
  "dash.apiInfo": "API Info",
  "dash.unload": "Unload",
  "dash.logs": "LIVE LOGS",
  "dash.streaming": "Streaming",
  "dash.paused": "Paused",
  "dash.clear": "Clear",
  "dash.waitLogs": "Waiting for logs...",
  "dash.apiTitle": "API Usage Examples",
  "dash.apiModel": "Model",
  "dash.apiCurl": "cURL",
  "dash.apiPython": "Python",
  "dash.apiJs": "JavaScript",
  "dash.close": "Close",
  "dash.copied": "Copied!",

  // Marketplace
  "mp.title": "Marketplace",
  "mp.subtitle": "The model application market allows for quick loading and usage.",
  "mp.models": "Models",
  "mp.datasets": "Datasets",
  "mp.search": "Search...",
  "mp.trending": "Trending",
  "mp.recentlyUpdated": "Recently Updated",
  "mp.mostDownloads": "Most Downloads",
  "mp.mostLikes": "Most Likes",
  "mp.loading": "Loading...",
  "mp.noModels": "No models found.",
  "mp.noDatasets": "No datasets found.",
  "mp.prev": "Previous",
  "mp.next": "Next",
  "mp.page": "Page {0} of {1}",
  "mp.done": "Done!",
  "mp.downloaded": "Downloaded",
  "mp.failed": "Failed",
  "mp.pulling": "Pulling...",
  "mp.download": "Download",
  "mp.sortBy": "Sort by",
  "mp.filter": "Filter",
  "mp.updatedAt": "Updated: {0}",
  "mp.viewer": "Viewer",

  // Settings
  "nav.settings": "settings",
  "settings.title": "settings",
  "settings.subtitle": "This page offers a unified settings hub for the product.",
  "settings.modelLocation": "Model location",
  "settings.modelLocationDesc": "Location where models are stored.",
  "settings.contextLength": "Context length",
  "settings.contextLengthDesc": "Context length determines how much of your conversation local LLMs can remember and use to generate responses.",
  "settings.versionInfo": "Version Information",
  "settings.resetDefaults": "Reset to defaults",
  "settings.language": "Language",
  "settings.languageDesc": "Select the display language for the interface.",

  // Library
  "lib.title": "Library",
  "lib.subtitle": "Existing model management",
  "lib.all": "All",
  "lib.modelName": "Model Name",
  "lib.format": "Format",
  "lib.fileSize": "File size",
  "lib.dateTime": "Date & Time",
  "lib.operation": "Operation",
  "lib.noModels": "No models found. Pull models from the Marketplace.",
  "lib.delete": "Delete",
  "lib.deleteConfirm": "Delete model \"{0}\"?",
  "lib.running": "Running",
  "lib.converting": "Converting...",
  "lib.loadingModel": "Loading...",
  "lib.run": "Run",
  "lib.failedLoad": "Failed to load model",

  // Chat
  "chat.newChat": "New Chat",
  "chat.chat": "Chat",
  "chat.settings": "Settings",
  "chat.clearHistory": "Clear history",
  "chat.clearConfirm": "Clear all messages in this session?",
  "chat.suggestion": "Make a Suggestion",
  "chat.systemPrompt": "System Prompt",
  "chat.placeholder": "Placeholder text...",
  "chat.resetDefaults": "Reset Defaults",
  "chat.startConv": "Start a conversation...",
  "chat.askImage": "Ask about an image...",
  "chat.askHelp": "How can I help you?",
  "chat.uploadImage": "Upload image",
  "chat.send": "Send",
  "chat.stop": "Stop",
  "chat.failedResp": "Failed to get response. Is the model loaded?",
  "chat.noMmproj": "This vision model requires a multimodal projector (mmproj) file to process images. Images will be ignored.",
};

const zh: Record<string, string> = {
  // Nav
  "nav.dashboard": "仪表盘",
  "nav.marketplace": "模型市场",
  "nav.library": "模型库",
  "nav.chat": "对话",

  // Dashboard
  "dash.resource": "资源使用情况",
  "dash.updates": "每 3 秒更新",
  "dash.cpu": "CPU 使用率",
  "dash.ram": "内存使用率",
  "dash.gpu": "GPU 显存",
  "dash.na": "N/A",
  "dash.active": "运行中的推理模型",
  "dash.noModels": "当前没有运行中的模型。",
  "dash.modelName": "模型名称",
  "dash.expiresAt": "过期时间",
  "dash.actions": "操作",
  "dash.format": "格式",
  "dash.apiInfo": "API 调用",
  "dash.unload": "卸载",
  "dash.logs": "实时日志",
  "dash.streaming": "传输中",
  "dash.paused": "已暂停",
  "dash.clear": "清除",
  "dash.waitLogs": "等待日志...",
  "dash.apiTitle": "API 调用示例",
  "dash.apiModel": "模型",
  "dash.apiCurl": "cURL",
  "dash.apiPython": "Python",
  "dash.apiJs": "JavaScript",
  "dash.close": "关闭",
  "dash.copied": "已复制!",

  // Marketplace
  "mp.title": "模型市场",
  "mp.subtitle": "模型应用市场，支持快速加载和使用。",
  "mp.models": "模型",
  "mp.datasets": "数据集",
  "mp.search": "搜索...",
  "mp.trending": "热门",
  "mp.recentlyUpdated": "最近更新",
  "mp.mostDownloads": "下载最多",
  "mp.mostLikes": "最多收藏",
  "mp.loading": "加载中...",
  "mp.noModels": "未找到模型。",
  "mp.noDatasets": "未找到数据集。",
  "mp.prev": "上一页",
  "mp.next": "下一页",
  "mp.page": "第 {0} 页 / 共 {1} 页",
  "mp.done": "完成！",
  "mp.downloaded": "已下载",
  "mp.failed": "失败",
  "mp.pulling": "拉取中...",
  "mp.download": "下载",
  "mp.sortBy": "排序",
  "mp.filter": "筛选",
  "mp.updatedAt": "更新时间: {0}",
  "mp.viewer": "浏览",

  // Settings
  "nav.settings": "设置",
  "settings.title": "设置",
  "settings.subtitle": "此页面提供产品的统一设置中心。",
  "settings.modelLocation": "模型存储位置",
  "settings.modelLocationDesc": "模型文件的存储路径。",
  "settings.contextLength": "上下文长度",
  "settings.contextLengthDesc": "上下文长度决定了本地 LLM 可以记住多少对话内容并用于生成回复。",
  "settings.versionInfo": "版本信息",
  "settings.resetDefaults": "恢复默认设置",
  "settings.language": "语言",
  "settings.languageDesc": "选择界面显示语言。",

  // Library
  "lib.title": "模型库",
  "lib.subtitle": "已有模型管理",
  "lib.all": "全部",
  "lib.modelName": "模型名称",
  "lib.format": "格式",
  "lib.fileSize": "文件大小",
  "lib.dateTime": "日期时间",
  "lib.operation": "操作",
  "lib.noModels": "未找到模型。请从模型市场拉取模型。",
  "lib.delete": "删除",
  "lib.deleteConfirm": "确定删除模型 \"{0}\" 吗？",
  "lib.running": "运行中",
  "lib.converting": "转换中...",
  "lib.loadingModel": "加载中...",
  "lib.run": "运行",
  "lib.failedLoad": "加载模型失败",

  // Chat
  "chat.newChat": "新对话",
  "chat.chat": "对话",
  "chat.settings": "设置",
  "chat.clearHistory": "清除历史",
  "chat.clearConfirm": "确定清除当前会话的所有消息吗？",
  "chat.suggestion": "参数调节",
  "chat.systemPrompt": "系统提示词",
  "chat.placeholder": "请输入提示词...",
  "chat.resetDefaults": "重置默认",
  "chat.startConv": "开始一段对话...",
  "chat.askImage": "描述一下这张图片...",
  "chat.askHelp": "有什么可以帮你的？",
  "chat.uploadImage": "上传图片",
  "chat.send": "发送",
  "chat.stop": "停止",
  "chat.failedResp": "获取回复失败，模型是否已加载？",
  "chat.noMmproj": "此视觉模型需要多模态投影（mmproj）文件来处理图片，图片将被忽略。",
};

const translations: Record<Locale, Record<string, string>> = { en, zh };

export function t(key: string, ...args: (string | number)[]): string {
  const str = translations[locale.value]?.[key] || en[key] || key;
  if (args.length === 0) return str;
  return str.replace(/\{(\d+)\}/g, (_, i) => String(args[Number(i)]));
}
