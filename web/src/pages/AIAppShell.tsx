import { useEffect, useMemo, useRef, useState } from "preact/hooks";
import { FitAddon } from "@xterm/addon-fit";
import { Terminal } from "@xterm/xterm";
import "@xterm/xterm/css/xterm.css";
import { getTags, openAIApp, type ModelInfo } from "../api/client";
import { locale, t } from "../i18n";

type ConnectionState = "connecting" | "connected" | "disconnected" | "exited";
const claudeCodeAppId = "claude-code";

interface ShellControlMessage {
  type: string;
  session_id?: string;
  app_id?: string;
  title?: string;
  model_id?: string;
  work_dir?: string;
  exit_code?: number;
  error?: string;
}

function shellWebSocketURL(sessionId: string): string {
  const protocol = location.protocol === "https:" ? "wss:" : "ws:";
  return `${protocol}//${location.host}/api/apps/shell/${encodeURIComponent(sessionId)}/ws`;
}

function normalizeShellModels(models: ModelInfo[]): ModelInfo[] {
  const seen = new Set<string>();
  const out: ModelInfo[] = [];
  for (const model of models) {
    const modelId = model.model?.trim();
    if (!modelId || seen.has(modelId)) {
      continue;
    }
    seen.add(modelId);
    out.push(model);
  }
  return out;
}

function formatShellModelLabel(model: ModelInfo): string {
  const name = model.display_name || model.model;
  const source = model.source === "cloud" ? t("aiApps.modelSourceCloud") : t("aiApps.modelSourceLocal");
  return `${name} (${source})`;
}

async function closeShellSession(sessionId: string): Promise<void> {
  if (!sessionId) {
    return;
  }
  await fetch(`/api/apps/shell/${encodeURIComponent(sessionId)}/close`, {
    method: "POST",
    keepalive: true,
  }).catch(() => {});
}

export function AIAppShell() {
  void locale.value;

  const containerRef = useRef<HTMLDivElement>(null);
  const copyResetRef = useRef<number | null>(null);
  const sessionId = useMemo(() => new URLSearchParams(location.search).get("session_id")?.trim() || "", []);
  const queryAppId = useMemo(() => new URLSearchParams(location.search).get("app_id")?.trim() || "", []);
  const [title, setTitle] = useState("");
  const [modelId, setModelId] = useState("");
  const [appId, setAppId] = useState(queryAppId);
  const [error, setError] = useState("");
  const [state, setState] = useState<ConnectionState>("connecting");
  const [exitCode, setExitCode] = useState<number | null>(null);
  const [workDir, setWorkDir] = useState("");
  const [workDirInput, setWorkDirInput] = useState("");
  const [models, setModels] = useState<ModelInfo[]>([]);
  const [modelsLoading, setModelsLoading] = useState(false);
  const [selectedModel, setSelectedModel] = useState("");
  const [applyingLaunchConfig, setApplyingLaunchConfig] = useState(false);
  const [copiedModel, setCopiedModel] = useState(false);
  const exitSeenRef = useRef(false);

  useEffect(() => {
    return () => {
      if (copyResetRef.current !== null) {
        window.clearTimeout(copyResetRef.current);
      }
    };
  }, []);

  useEffect(() => {
    if (!sessionId) {
      setError(t("aiApps.shellSessionMissing"));
      setState("disconnected");
      return;
    }
    if (!containerRef.current) {
      return;
    }

    const terminal = new Terminal({
      cursorBlink: true,
      fontFamily: "ui-monospace, SFMono-Regular, SF Mono, Menlo, Consolas, monospace",
      fontSize: 13,
      lineHeight: 1.35,
      theme: {
        background: "#020617",
        foreground: "#E5E7EB",
        cursor: "#818CF8",
        selectionBackground: "rgba(129, 140, 248, 0.28)",
      },
      convertEol: false,
      allowProposedApi: false,
      scrollback: 5000,
    });
    const fitAddon = new FitAddon();
    terminal.loadAddon(fitAddon);
    terminal.open(containerRef.current);
    fitAddon.fit();
    terminal.focus();

    const ws = new WebSocket(shellWebSocketURL(sessionId));
    ws.binaryType = "arraybuffer";
    const encoder = new TextEncoder();

    const sendResize = () => {
      fitAddon.fit();
      if (ws.readyState !== WebSocket.OPEN) {
        return;
      }
      ws.send(JSON.stringify({
        type: "resize",
        cols: terminal.cols,
        rows: terminal.rows,
      }));
    };

    const resizeHandler = () => {
      sendResize();
    };

    const inputDisposable = terminal.onData((data) => {
      if (ws.readyState === WebSocket.OPEN) {
        ws.send(encoder.encode(data));
      }
    });

    ws.onopen = () => {
      setState("connected");
      setError("");
      sendResize();
    };

    ws.onmessage = (event) => {
      if (typeof event.data === "string") {
        let message: ShellControlMessage | null = null;
        try {
          message = JSON.parse(event.data) as ShellControlMessage;
        } catch {
          message = null;
        }
        if (!message) {
          return;
        }

        if (message.type === "ready") {
          setTitle(message.title || "");
          setAppId(message.app_id || "");
          setModelId(message.model_id || "");
          setWorkDir(message.work_dir || "");
          setWorkDirInput(message.work_dir || "");
          document.title = message.title ? `${message.title} · CSGHub Lite` : "CSGHub Lite";
          return;
        }

        if (message.type === "exit") {
          exitSeenRef.current = true;
          setState("exited");
          setExitCode(typeof message.exit_code === "number" ? message.exit_code : 0);
          if (message.error) {
            setError(message.error);
          }
          terminal.writeln("");
          terminal.writeln(`\x1b[90m[${t("aiApps.shellExitNotice", String(message.exit_code ?? 0))}]\x1b[0m`);
          return;
        }

        return;
      }

      const chunk = event.data instanceof ArrayBuffer
        ? new Uint8Array(event.data)
        : new Uint8Array();
      if (chunk.byteLength > 0) {
        terminal.write(chunk);
      }
    };

    ws.onerror = () => {
      if (!exitSeenRef.current) {
        setError(t("aiApps.shellConnectionFailed"));
      }
    };

    ws.onclose = () => {
      if (!exitSeenRef.current) {
        setState("disconnected");
      }
    };

    window.addEventListener("resize", resizeHandler);

    return () => {
      window.removeEventListener("resize", resizeHandler);
      inputDisposable.dispose();
      ws.close();
      terminal.dispose();
    };
  }, [sessionId]);

  useEffect(() => {
    if (appId !== claudeCodeAppId) {
      setModels([]);
      setModelsLoading(false);
      return;
    }

    let disposed = false;
    setModelsLoading(true);

    getTags()
      .then((items) => {
        if (disposed) return;
        setModels(normalizeShellModels(items));
      })
      .catch(() => {
        if (disposed) return;
        setModels([]);
      })
      .finally(() => {
        if (!disposed) {
          setModelsLoading(false);
        }
      });

    return () => {
      disposed = true;
    };
  }, [appId]);

  useEffect(() => {
    if (appId !== claudeCodeAppId) {
      setSelectedModel("");
      return;
    }

    setSelectedModel((current) => {
      if (modelId && models.some((item) => item.model === modelId)) {
        return modelId;
      }
      if (current && models.some((item) => item.model === current)) {
        return current;
      }
      return models[0]?.model || "";
    });
  }, [appId, modelId, models]);

  const shellTitle = title || t("aiApps.shellTitle");
  const statusLabel = state === "connected"
    ? t("aiApps.shellConnected")
    : state === "exited"
      ? t("aiApps.shellExited")
      : state === "disconnected"
        ? t("aiApps.shellDisconnected")
        : t("aiApps.shellConnecting");
  const statusClass = state === "connected"
    ? "bg-emerald-500/15 text-emerald-300 border-emerald-500/30"
    : state === "exited"
      ? "bg-slate-500/15 text-slate-300 border-slate-500/30"
      : "bg-amber-500/15 text-amber-200 border-amber-500/30";

  const canConfigureClaudeShell = appId === claudeCodeAppId;
  const trimmedWorkDir = workDirInput.trim();
  const launchConfigChanged = selectedModel !== modelId || trimmedWorkDir !== workDir;
  const applyLaunchConfigDisabled = !canConfigureClaudeShell ||
    modelsLoading ||
    applyingLaunchConfig ||
    !selectedModel ||
    !trimmedWorkDir ||
    !launchConfigChanged;

  const handleApplyLaunchConfig = async () => {
    if (!canConfigureClaudeShell || applyLaunchConfigDisabled) {
      return;
    }

    setApplyingLaunchConfig(true);
    setError("");
    try {
      const { url } = await openAIApp(claudeCodeAppId, selectedModel, trimmedWorkDir);
      await closeShellSession(sessionId);
      location.replace(url);
      return;
    } catch (err) {
      setError((err as Error).message || t("aiApps.openFailed"));
    } finally {
      setApplyingLaunchConfig(false);
    }
  };

  const handleCloseWindow = () => {
    void closeShellSession(sessionId);
    window.close();
    window.setTimeout(() => {
      if (!window.closed) {
        location.href = "/ai-apps";
      }
    }, 50);
  };

  const handleCopyModel = async () => {
    if (!modelId) {
      return;
    }
    try {
      await navigator.clipboard.writeText(modelId);
      setCopiedModel(true);
      if (copyResetRef.current !== null) {
        window.clearTimeout(copyResetRef.current);
      }
      copyResetRef.current = window.setTimeout(() => {
        setCopiedModel(false);
        copyResetRef.current = null;
      }, 1500);
    } catch {
      // Ignore clipboard failures so the shell UI keeps working.
    }
  };

  return (
    <div class="min-h-screen bg-slate-950 text-slate-100 flex flex-col">
      <div class="border-b border-slate-800 px-5 py-4 flex items-center justify-between gap-4">
        <div class="min-w-0">
          <div class="flex items-center gap-3 flex-wrap">
            <h1 class="text-base font-semibold text-white truncate">{shellTitle}</h1>
            <span class={`inline-flex items-center rounded-full border px-2.5 py-1 text-xs font-medium ${statusClass}`}>
              {statusLabel}
            </span>
          </div>
          <div class="mt-1 flex items-center gap-3 flex-wrap text-xs text-slate-400">
            {appId && <span>{appId}</span>}
            {modelId && <span>{t("aiApps.shellModel")}: {modelId}</span>}
            {workDir && <span>{t("aiApps.shellDirectory")}: {workDir}</span>}
            {state === "exited" && exitCode !== null && <span>{t("aiApps.shellExitCode", String(exitCode))}</span>}
          </div>
        </div>
        <div class="flex items-center gap-2 flex-wrap justify-end">
          {canConfigureClaudeShell && (
            <>
              <div class="relative min-w-[260px] max-w-[340px]">
                <select
                  value={selectedModel}
                  onChange={(e) => setSelectedModel((e.currentTarget as HTMLSelectElement).value)}
                  disabled={modelsLoading || applyingLaunchConfig || models.length === 0}
                  class={`appearance-none w-full rounded-lg border bg-slate-900 pl-3 pr-9 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:border-transparent ${
                    modelsLoading || applyingLaunchConfig || models.length === 0
                      ? "border-slate-700 text-slate-500"
                      : "border-slate-700 text-slate-100"
                  }`}
                  aria-label={t("aiApps.model")}
                >
                  {modelsLoading ? (
                    <option value="">{t("aiApps.modelLoading")}</option>
                  ) : models.length === 0 ? (
                    <option value="">{t("aiApps.modelDefault")}</option>
                  ) : (
                    models.map((model) => (
                      <option key={model.model} value={model.model}>
                        {formatShellModelLabel(model)}
                      </option>
                    ))
                  )}
                </select>
                <svg class="absolute right-3 top-1/2 -translate-y-1/2 w-4 h-4 text-slate-400 pointer-events-none" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M19 9l-7 7-7-7" />
                </svg>
              </div>
              <input
                type="text"
                value={workDirInput}
                onInput={(e) => setWorkDirInput((e.currentTarget as HTMLInputElement).value)}
                placeholder={t("aiApps.shellDirectoryPlaceholder")}
                disabled={applyingLaunchConfig}
                class={`min-w-[300px] max-w-[420px] rounded-lg border bg-slate-900 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:border-transparent ${
                  applyingLaunchConfig
                    ? "border-slate-700 text-slate-500"
                    : "border-slate-700 text-slate-100 placeholder:text-slate-500"
                }`}
                aria-label={t("aiApps.shellDirectory")}
              />
              <button
                onClick={handleApplyLaunchConfig}
                disabled={applyLaunchConfigDisabled}
                class={`rounded-lg border px-3 py-2 text-sm transition-colors ${
                  applyLaunchConfigDisabled
                    ? "border-slate-800 text-slate-500 cursor-not-allowed"
                    : "border-indigo-500/40 text-indigo-200 hover:bg-indigo-500/10"
                }`}
              >
                {applyingLaunchConfig ? t("aiApps.shellApplyingLaunch") : t("aiApps.shellApplyLaunch")}
              </button>
            </>
          )}
          <button
            onClick={() => location.reload()}
            class="rounded-lg border border-slate-700 px-3 py-2 text-sm text-slate-200 hover:bg-slate-900 transition-colors"
          >
            {t("aiApps.shellReconnect")}
          </button>
          <button
            onClick={handleCloseWindow}
            class="rounded-lg bg-indigo-600 px-3 py-2 text-sm font-medium text-white hover:bg-indigo-500 transition-colors"
          >
            {t("aiApps.close")}
          </button>
        </div>
      </div>

      {error && (
        <div class="px-5 py-3 border-b border-red-900/40 bg-red-950/40 text-sm text-red-200">
          {error}
        </div>
      )}

      {canConfigureClaudeShell && modelId && (
        <div class="border-b border-slate-800 bg-slate-950/80 px-5 py-3">
          <div class="flex items-center justify-between gap-3 flex-wrap">
            <div class="min-w-0 flex-1">
              <div class="text-[11px] font-medium uppercase tracking-wide text-slate-400">
                {t("aiApps.shellModel")}
              </div>
              <input
                type="text"
                readOnly
                value={modelId}
                onFocus={(e) => (e.currentTarget as HTMLInputElement).select()}
                class="mt-1 w-full rounded-lg border border-slate-800 bg-slate-900 px-3 py-2 font-mono text-sm text-slate-100 focus:outline-none focus:ring-2 focus:ring-indigo-500"
                aria-label={t("aiApps.shellModel")}
              />
            </div>
            <button
              onClick={() => void handleCopyModel()}
              class={`rounded-lg border px-3 py-2 text-sm transition-colors ${
                copiedModel
                  ? "border-emerald-500/40 text-emerald-200"
                  : "border-slate-700 text-slate-200 hover:bg-slate-900"
              }`}
              title={t("chat.copyModel")}
            >
              {copiedModel ? t("dash.copied") : t("chat.copyModel")}
            </button>
          </div>
        </div>
      )}

      {!sessionId ? (
        <div class="flex-1 flex items-center justify-center px-6 text-sm text-slate-400">
          {t("aiApps.shellSessionMissing")}
        </div>
      ) : (
        <div class="flex-1 min-h-0">
          <div ref={containerRef} class="h-full w-full p-3" />
        </div>
      )}
    </div>
  );
}
