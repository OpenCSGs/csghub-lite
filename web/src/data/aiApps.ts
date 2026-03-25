import type { Locale } from "../i18n";

export type AIAppCategory = "coding" | "automation" | "documents-rag";
export type AIAppInstallMode = "script" | "npm" | "docker";
export type AIAppProgressMode = "percent" | "indeterminate";
export type AIAppStatus = "idle" | "installing" | "uninstalling" | "installed" | "failed" | "disabled";

export interface LocalizedText {
  en: string;
  zh: string;
}

export interface AIAppCatalogEntry {
  id: string;
  name: string;
  siteLabel: string;
  website: string;
  detailsUrl: string;
  icon: string;
  category: AIAppCategory;
  description: LocalizedText;
  installMode: AIAppInstallMode;
  progressMode: AIAppProgressMode;
  installHint: LocalizedText;
  cnInstallHint: LocalizedText;
  commandPreview: string;
  liveLogsReady: boolean;
  plannedSteps: LocalizedText[];
  status: AIAppStatus;
  progress?: number;
  statusText: LocalizedText;
}

export interface AIAppRuntimeState {
  status: AIAppStatus;
  phase: string;
  progressMode: AIAppProgressMode;
  progress?: number;
  supported: boolean;
  disabled: boolean;
  liveLogsReady: boolean;
  installPath?: string;
  version?: string;
  logPath?: string;
  lastError?: string;
  logLines: string[];
}

export const aiAppCategoryOptions: Array<{ id: "all" | AIAppCategory; label: LocalizedText }> = [
  { id: "all", label: { en: "All", zh: "全部" } },
  { id: "coding", label: { en: "Coding", zh: "编程" } },
  { id: "automation", label: { en: "Automation", zh: "自动化" } },
  { id: "documents-rag", label: { en: "Documents & RAG", zh: "文档与 RAG" } },
];

export const aiAppsCatalog: AIAppCatalogEntry[] = [
  {
    id: "claude-code",
    name: "Claude Code",
    siteLabel: "@claude.ai",
    website: "https://claude.ai",
    detailsUrl: "https://docs.anthropic.com/en/docs/claude-code/setup",
    icon: "/apps/claude-code.svg",
    category: "coding",
    description: {
      en: "Anthropic's agentic coding tool that can read, edit, and run code in your working directory.",
      zh: "Anthropic 的智能编程工具，可在你的工作目录中读取、修改并执行代码。",
    },
    installMode: "script",
    progressMode: "indeterminate",
    installHint: {
      en: "Preferred upstream installer for macOS, Linux, and WSL.",
      zh: "适用于 macOS、Linux 和 WSL 的官方脚本安装方式。",
    },
    cnInstallHint: {
      en: "For domestic environments, preconfigure a system proxy or mirror the installer script internally before execution.",
      zh: "国内环境建议预先配置系统代理，或将安装脚本镜像到内网后再执行。",
    },
    commandPreview: "curl -fsSL https://git-devops.opencsg.com/opensource/apps/-/raw/main/claude-code/install.sh | bash",
    liveLogsReady: true,
    plannedSteps: [
      {
        en: "Resolve the latest installer script and verify shell compatibility.",
        zh: "解析最新安装脚本并检查当前 shell 环境是否兼容。",
      },
      {
        en: "Download Claude Code binaries and place them on the executable path.",
        zh: "下载 Claude Code 二进制并写入可执行路径。",
      },
      {
        en: "Open the first interactive session to finish sign-in and permissions.",
        zh: "首次打开交互会话，完成登录与权限确认。",
      },
    ],
    status: "idle",
    statusText: {
      en: "Ready to install latest",
      zh: "可安装最新版本",
    },
  },
  {
    id: "open-code",
    name: "OpenCode",
    siteLabel: "@opencode.ai",
    website: "https://opencode.ai",
    detailsUrl: "https://opencode.ai/docs/cli/",
    icon: "/apps/open-code.svg",
    category: "coding",
    description: {
      en: "An open-source AI coding agent that works in the terminal, desktop app, and editor extensions.",
      zh: "开源 AI 编码代理，支持终端、桌面应用和 IDE 扩展等多种使用方式。",
    },
    installMode: "npm",
    progressMode: "percent",
    installHint: {
      en: "Install the global CLI with npm, or switch to the shell installer when npm is unavailable.",
      zh: "优先使用 npm 全局安装，也可在 npm 不可用时切换到官方脚本安装。",
    },
    cnInstallHint: {
      en: "For domestic environments, prefer npmmirror and cache npm dependencies close to the target machine.",
      zh: "国内环境建议优先走 npmmirror，并将 npm 依赖缓存到离目标机器更近的镜像源。",
    },
    commandPreview: "curl -fsSL https://git-devops.opencsg.com/opensource/apps/-/raw/main/open-code/install.sh | bash",
    liveLogsReady: true,
    plannedSteps: [
      {
        en: "Resolve package metadata from the configured npm registry.",
        zh: "从当前配置的 npm registry 解析包元数据。",
      },
      {
        en: "Download the CLI package and unpack its runtime dependencies.",
        zh: "下载 CLI 包并展开运行时依赖。",
      },
      {
        en: "Verify the global binary and open the first session.",
        zh: "校验全局命令后启动首次会话。",
      },
    ],
    status: "idle",
    statusText: {
      en: "Ready to install latest",
      zh: "可安装最新版本",
    },
  },
  {
    id: "openclaw",
    name: "OpenClaw",
    siteLabel: "@openclaw.ai",
    website: "https://openclaw.ai",
    detailsUrl: "https://docs.openclaw.ai/install",
    icon: "/apps/openclaw.svg",
    category: "automation",
    description: {
      en: "A personal AI assistant that runs on your own devices and bridges messaging services, workflows, and local control.",
      zh: "运行在你自己设备上的个人 AI 助手，可连接消息渠道、自动化工作流与本地控制能力。",
    },
    installMode: "script",
    progressMode: "indeterminate",
    installHint: {
      en: "Use the official installer script first, then finish onboarding and daemon setup.",
      zh: "优先使用官方安装脚本，再完成引导配置与守护进程安装。",
    },
    cnInstallHint: {
      en: "For domestic environments, prepare a proxy or mirror the installer resources internally before running the setup.",
      zh: "国内环境建议先准备代理，或将安装资源镜像到内网后再执行安装。",
    },
    commandPreview: "curl -fsSL https://git-devops.opencsg.com/opensource/apps/-/raw/main/openclaw/install.sh | bash",
    liveLogsReady: true,
    plannedSteps: [
      {
        en: "Run the installer script and ensure a supported Node runtime is available.",
        zh: "运行安装脚本，并确保当前环境具备受支持的 Node 运行时。",
      },
      {
        en: "Complete onboarding, authentication, and local daemon installation.",
        zh: "完成引导配置、鉴权以及本地守护进程安装。",
      },
      {
        en: "Verify gateway status and open the dashboard to finish setup.",
        zh: "检查 gateway 状态，并打开 dashboard 完成初始化。",
      },
    ],
    status: "idle",
    statusText: {
      en: "Ready to install",
      zh: "可安装",
    },
  },
  {
    id: "codex",
    name: "Codex",
    siteLabel: "@openai.com",
    website: "https://developers.openai.com/codex/cli",
    detailsUrl: "https://developers.openai.com/codex/cli",
    icon: "/apps/codex.svg",
    category: "coding",
    description: {
      en: "OpenAI's local coding agent CLI for reviewing code, editing files, and automating terminal workflows.",
      zh: "OpenAI 的本地编码代理 CLI，可用于代码审查、文件修改和终端自动化工作流。",
    },
    installMode: "npm",
    progressMode: "indeterminate",
    installHint: {
      en: "Officially installed as a global npm package.",
      zh: "官方推荐通过 npm 全局包方式安装。",
    },
    cnInstallHint: {
      en: "When running from mainland China, use a domestic npm mirror first and keep OpenAI authentication configured separately.",
      zh: "中国大陆环境建议先切换国内 npm 镜像，同时单独准备 OpenAI 登录或鉴权配置。",
    },
    commandPreview: "curl -fsSL https://git-devops.opencsg.com/opensource/apps/-/raw/main/codex/install.sh | bash",
    liveLogsReady: true,
    plannedSteps: [
      {
        en: "Resolve the package version and download npm tarballs.",
        zh: "解析包版本并下载 npm tarball。",
      },
      {
        en: "Install the global binary and required node modules.",
        zh: "安装全局命令与所需 node modules。",
      },
      {
        en: "Start Codex once to finish authentication.",
        zh: "首次运行 Codex 完成登录与认证。",
      },
    ],
    status: "idle",
    statusText: {
      en: "Ready to install",
      zh: "可安装",
    },
  },
  {
    id: "dify",
    name: "Dify",
    siteLabel: "@dify.ai",
    website: "https://dify.ai",
    detailsUrl: "https://docs.dify.ai/getting-started/install-self-hosted",
    icon: "/apps/dify.svg",
    category: "automation",
    description: {
      en: "An open-source platform for building agentic workflows, chat applications, and LLM orchestration services.",
      zh: "面向工作流、聊天应用和 LLM 编排场景的开源平台。",
    },
    installMode: "docker",
    progressMode: "indeterminate",
    installHint: {
      en: "Use the official Docker Compose deployment for a self-hosted installation.",
      zh: "首版建议采用官方 Docker Compose 方式进行自托管部署。",
    },
    cnInstallHint: {
      en: "Domestic deployments should prepare Git mirrors and Docker registry acceleration ahead of time.",
      zh: "国内部署建议提前准备 Git 镜像与 Docker 镜像加速配置。",
    },
    commandPreview: [
      "git clone https://github.com/langgenius/dify.git",
      "cd dify/docker",
      "cp .env.example .env",
      "docker compose up -d",
    ].join("\n"),
    liveLogsReady: true,
    plannedSteps: [
      {
        en: "Clone the deployment repository and prepare the .env file.",
        zh: "克隆部署仓库并准备 .env 配置文件。",
      },
      {
        en: "Pull required service images and boot the compose stack.",
        zh: "拉取所需服务镜像并拉起 compose 栈。",
      },
      {
        en: "Wait for the API, worker, and web services to become healthy.",
        zh: "等待 API、worker 和 web 服务进入健康状态。",
      },
    ],
    status: "disabled",
    statusText: {
      en: "Disabled in AI Apps",
      zh: "应用页暂不支持",
    },
  },
  {
    id: "anythingllm",
    name: "AnythingLLM",
    siteLabel: "@anythingllm.com",
    website: "https://anythingllm.com",
    detailsUrl: "https://docs.anythingllm.com/installation-docker/overview",
    icon: "/apps/anythingllm.svg",
    category: "documents-rag",
    description: {
      en: "An all-in-one AI application for chat with documents, agents, and private RAG workflows.",
      zh: "一体化 AI 应用，支持文档问答、Agent 和私有化 RAG 工作流。",
    },
    installMode: "docker",
    progressMode: "indeterminate",
    installHint: {
      en: "The official self-hosted path uses Docker images published by Mintplex Labs.",
      zh: "官方自托管路径基于 Mintplex Labs 发布的 Docker 镜像。",
    },
    cnInstallHint: {
      en: "Domestic environments should prepare Docker Hub acceleration or a mirrored image registry before installation.",
      zh: "国内环境建议先配置 Docker Hub 加速或镜像代理，再执行安装。",
    },
    commandPreview: [
      "docker pull mintplexlabs/anythingllm:latest",
      "docker run -d -p 3001:3001 --cap-add SYS_ADMIN \\",
      "  -v ${STORAGE_LOCATION}:/app/server/storage \\",
      "  mintplexlabs/anythingllm:latest",
    ].join("\n"),
    liveLogsReady: true,
    plannedSteps: [
      {
        en: "Pull the official image and prepare a persistent storage mount.",
        zh: "拉取官方镜像并准备持久化存储挂载目录。",
      },
      {
        en: "Start the container with the required port and capability flags.",
        zh: "带上端口与能力参数启动容器。",
      },
      {
        en: "Verify the web UI and storage path before exposing it to users.",
        zh: "在对外开放前，校验 Web UI 与存储路径是否可用。",
      },
    ],
    status: "disabled",
    statusText: {
      en: "Disabled in AI Apps",
      zh: "应用页暂不支持",
    },
  },
];

export const initialAIAppStates = aiAppsCatalog.reduce<Record<string, AIAppRuntimeState>>((acc, app) => {
  acc[app.id] = {
    status: app.status,
    phase: app.status === "disabled" ? "docker_disabled" : "ready",
    progressMode: app.progressMode,
    progress: app.status === "disabled" ? 0 : undefined,
    supported: app.installMode !== "docker",
    disabled: app.installMode === "docker",
    liveLogsReady: app.liveLogsReady,
    logLines: [],
  };
  return acc;
}, {});

export function createAIAppStateSnapshot(): Record<string, AIAppRuntimeState> {
  return Object.fromEntries(
    Object.entries(initialAIAppStates).map(([id, state]) => [id, { ...state, logLines: [...state.logLines] }])
  ) as Record<string, AIAppRuntimeState>;
}

export function getLocalizedText(text: LocalizedText, currentLocale: Locale): string {
  return currentLocale === "zh" ? text.zh : text.en;
}
