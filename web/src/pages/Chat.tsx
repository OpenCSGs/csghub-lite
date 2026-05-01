import { useEffect, useRef } from "preact/hooks";
import { signal, computed } from "@preact/signals";
import { getTags, getPs, streamChat, getCloudAuthStatus, saveCloudToken } from "../api/client";
import type { ModelInfo, ChatMessage, ContentPart, CloudAuthStatus } from "../api/client";
import { t, locale } from "../i18n";
import { parseReasoningText } from "../reasoning";

interface Session {
  id: string;
  title: string;
  messages: ChatMessage[];
  numCtx?: number;
  numParallel?: number;
}

const availableModels = signal<ModelInfo[]>([]);
const selectedModelKey = signal("");
const sessions = signal<Session[]>(loadSessions());
const activeSessionId = signal(sessions.value[0]?.id || "");
const inputText = signal("");
const isGenerating = signal(false);
const showSettings = signal(false);
const showCloudAuthDialog = signal(false);
const cloudAuth = signal<CloudAuthStatus | null>(null);
const cloudTokenInput = signal("");
const cloudAuthError = signal("");
const isSavingCloudToken = signal(false);

function hasCloudAuth(status: CloudAuthStatus | null | undefined): boolean {
  return status?.authenticated ?? status?.has_token ?? false;
}

const systemPrompt = signal("");
const temperature = signal(0.95);
const topP = signal(0.75);
const maxTokens = signal(4096);

const streamingContent = signal("");
const chatError = signal("");
const pendingImages = signal<PendingImage[]>([]);
const contextStorageKey = "csghub.chat.num_ctx";
const contextLengthSteps = [4096, 8192, 16384, 32768, 65536, 131072, 262144];
const contextLengthLabels = ["4k", "8k", "16k", "32k", "64k", "128k", "256k"];
const parallelStorageKey = "csghub.chat.num_parallel";
const parallelSteps = [1, 2, 4, 8];
const selectedModelStorageKey = "csghub.chat.selected_model";

function modelKey(model: Pick<ModelInfo, "model" | "name" | "source">): string {
  return `${model.source || "local"}:${model.model || model.name}`;
}

function modelLabel(model: ModelInfo): string {
  const label = model.display_name || model.name;
  const tags: string[] = [];
  const source = model.source || "local";
  if (source === "cloud") tags.push(t("chat.cloud"));
  else tags.push(t("chat.local"));
  if (model.pipeline_tag === "image-text-to-text") tags.push("VL");
  return tags.length > 0 ? `${label} [${tags.join("] [")}]` : label;
}

function readSelectedModelKey(): string {
  try {
    return localStorage.getItem(selectedModelStorageKey) || "";
  } catch {
    return "";
  }
}

function saveSelectedModelKey(key: string) {
  try {
    if (key) {
      localStorage.setItem(selectedModelStorageKey, key);
    } else {
      localStorage.removeItem(selectedModelStorageKey);
    }
  } catch {
    /* ignore storage failures */
  }
}

const selectedModelInfo = computed(() =>
  availableModels.value.find((x) => modelKey(x) === selectedModelKey.value)
);

function setAvailableModels(models: ModelInfo[]) {
  availableModels.value = models;
  if (selectedModelKey.value && models.some((x) => modelKey(x) === selectedModelKey.value)) {
    return;
  }
  const savedModelKey = readSelectedModelKey();
  if (savedModelKey && models.some((x) => modelKey(x) === savedModelKey)) {
    selectedModelKey.value = savedModelKey;
    return;
  }
  if (models.length === 0) {
    selectedModelKey.value = "";
    return;
  }

  const localModels = models.filter((x) => (x.source || "local") === "local");
  const gguf = localModels.filter((x) => x.format === "gguf");
  const fallback = gguf[0] || localModels[0] || models[0];
  if (fallback) {
    selectedModelKey.value = modelKey(fallback);
    saveSelectedModelKey(selectedModelKey.value);
  }
}

const isVisionModel = computed(() => {
  const m = selectedModelInfo.value;
  return m?.pipeline_tag === "image-text-to-text" && (m?.source === "cloud" || m?.has_mmproj === true);
});

function normalizeImage(file: File): Promise<{ full: string; thumb: string }> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () => {
      const dataURL = reader.result as string;
      resolve({ full: dataURL, thumb: dataURL });
    };
    reader.onerror = () => reject(new Error("Failed to read file"));
    reader.readAsDataURL(file);
  });
}

interface PendingImage {
  full: string;
  thumb: string;
}

function readNumCtx(): number | undefined {
  try {
    const raw = localStorage.getItem(contextStorageKey);
    const n = Number(raw);
    if (Number.isFinite(n) && n >= 1024) return n;
  } catch {
    /* ignore */
  }
  return undefined;
}

function defaultNumCtx(): number {
  return readNumCtx() || 8192;
}

function normalizeNumCtx(v: number | undefined): number {
  if (!v) return defaultNumCtx();
  for (const s of contextLengthSteps) {
    if (s === v) return v;
  }
  return defaultNumCtx();
}

function readNumParallel(): number | undefined {
  try {
    const raw = localStorage.getItem(parallelStorageKey);
    const n = Number(raw);
    if (parallelSteps.includes(n)) return n;
  } catch {
    /* ignore */
  }
  return undefined;
}

function defaultNumParallel(): number {
  return readNumParallel() || 4;
}

function makeId(): string {
  if (typeof crypto !== "undefined" && crypto.randomUUID) {
    return crypto.randomUUID();
  }
  return "xxxx-xxxx-xxxx".replace(/x/g, () =>
    Math.floor(Math.random() * 16).toString(16)
  );
}

function loadSessions(): Session[] {
  try {
    const raw = localStorage.getItem("csghub-chat-sessions");
    if (raw) {
      const parsed = JSON.parse(raw);
      if (Array.isArray(parsed) && parsed.length > 0) {
        return parsed.map((s) => ({
          ...s,
          numCtx: normalizeNumCtx(s?.numCtx),
          numParallel: s?.numParallel || defaultNumParallel(),
        }));
      }
    }
  } catch { /* ignore */ }
  const s: Session = { id: makeId(), title: "New Chat", messages: [], numCtx: defaultNumCtx(), numParallel: defaultNumParallel() };
  return [s];
}

function stripImagesForStorage(s: Session[]): Session[] {
  return s.map((sess) => ({
    ...sess,
    messages: sess.messages.map((m) => {
      if (!Array.isArray(m.content)) return m;
      const textOnly = (m.content as ContentPart[])
        .filter((p) => p.type === "text")
        .map((p) => p.text || "")
        .join("");
      return { ...m, content: textOnly || "(image)" };
    }),
  }));
}

function saveSessions() {
  try {
    const safe = stripImagesForStorage(sessions.value);
    localStorage.setItem("csghub-chat-sessions", JSON.stringify(safe));
  } catch {
    /* quota exceeded or other storage error — non-fatal */
  }
}

function getActiveSession(): Session | undefined {
  return sessions.value.find((s) => s.id === activeSessionId.value);
}

export function Chat() {
  const messagesRef = useRef<HTMLDivElement>(null);
  const abortRef = useRef<AbortController | null>(null);
  void locale.value;

  const refreshCloudAuth = async (): Promise<CloudAuthStatus> => {
    const status = await getCloudAuthStatus();
    cloudAuth.value = status;
    return status;
  };

  const openCloudAuthDialog = async (message = "") => {
    cloudAuthError.value = message;
    showCloudAuthDialog.value = true;
    try {
      await refreshCloudAuth();
    } catch {
      /* ignore */
    }
  };

  const handleModelChange = (nextKey: string) => {
    selectedModelKey.value = nextKey;
    saveSelectedModelKey(nextKey);
    const model = availableModels.value.find((x) => modelKey(x) === nextKey);
    if (model?.source === "cloud" && !hasCloudAuth(cloudAuth.value)) {
      void openCloudAuthDialog(t("chat.cloudLoginRequired"));
    }
  };

  const handleOpenCloudLogin = () => {
    const url = cloudAuth.value?.login_url;
    if (url) {
      window.open(url, "_blank", "noopener,noreferrer");
    }
  };

  const handleOpenCloudTokenPage = () => {
    const url = cloudAuth.value?.access_token_url;
    if (url) {
      window.open(url, "_blank", "noopener,noreferrer");
    }
  };

  const handleSaveCloudToken = async () => {
    const token = cloudTokenInput.value.trim();
    if (!token) {
      cloudAuthError.value = t("chat.cloudTokenEmpty");
      return;
    }

    isSavingCloudToken.value = true;
    cloudAuthError.value = "";
    try {
      const status = await saveCloudToken(token);
      cloudAuth.value = status;
      if (!hasCloudAuth(status)) {
        cloudAuthError.value = t("chat.cloudLoginExpired");
        return;
      }
      try {
        setAvailableModels(await getTags({ refresh: true }));
      } catch {
        /* ignore */
      }
      cloudTokenInput.value = "";
      showCloudAuthDialog.value = false;
    } catch (e: any) {
      cloudAuthError.value = e?.message || t("chat.failedResp");
    } finally {
      isSavingCloudToken.value = false;
    }
  };

  useEffect(() => {
    getTags({ refresh: true }).then((m) => {
      setAvailableModels(m);
    }).catch(() => {});
    getPs().then((running) => {
      if (running.length > 0 && !selectedModelKey.value) {
        selectedModelKey.value = `local:${running[0].model || running[0].name}`;
        saveSelectedModelKey(selectedModelKey.value);
      }
    }).catch(() => {});
    getCloudAuthStatus().then((status) => {
      cloudAuth.value = status;
    }).catch(() => {});
  }, []);

  useEffect(() => {
    if (messagesRef.current) {
      messagesRef.current.scrollTop = messagesRef.current.scrollHeight;
    }
  }, [getActiveSession()?.messages.length, streamingContent.value]);

  const handleImageUpload = (e: Event) => {
    const files = (e.target as HTMLInputElement).files;
    if (!files) return;
    Array.from(files).forEach((file) => {
      normalizeImage(file)
        .then((img) => {
          pendingImages.value = [...pendingImages.value, img];
        })
        .catch((err) => {
          chatError.value = `${t("chat.failedResp")}: ${err?.message || err}`;
        });
    });
    (e.target as HTMLInputElement).value = "";
  };

  const removeImage = (idx: number) => {
    pendingImages.value = pendingImages.value.filter((_, i) => i !== idx);
  };

  const handleSend = async () => {
    const text = inputText.value.trim();
    const currentModel = selectedModelInfo.value;
    if (!text || !currentModel || isGenerating.value) return;

    if (currentModel.source === "cloud") {
      try {
        const status = cloudAuth.value || await refreshCloudAuth();
        if (!hasCloudAuth(status)) {
          await openCloudAuthDialog(t("chat.cloudLoginRequired"));
          return;
        }
      } catch {
        await openCloudAuthDialog(t("chat.cloudLoginRequired"));
        return;
      }
    }

    const session = getActiveSession();
    if (!session) {
      chatError.value = "No active session. Please create a new chat.";
      return;
    }

    const images = pendingImages.value;
    let userContent: ChatMessage["content"];
    let apiMessages: ChatMessage[];

    if (images.length > 0) {
      const displayParts: ContentPart[] = images.map((img) => ({
        type: "image_url" as const,
        image_url: { url: img.thumb },
      }));
      displayParts.push({ type: "text" as const, text });
      userContent = displayParts;

      const apiParts: ContentPart[] = images.map((img) => ({
        type: "image_url" as const,
        image_url: { url: img.full },
      }));
      apiParts.push({ type: "text" as const, text });

      apiMessages = [
        ...session.messages,
        { role: "user", content: apiParts },
      ];
    } else {
      userContent = text;
      apiMessages = [
        ...session.messages,
        { role: "user", content: text },
      ];
    }

    session.messages.push({ role: "user", content: userContent });
    if (session.messages.length === 1) {
      session.title = text.slice(0, 30) || "New Chat";
    }
    sessions.value = [...sessions.value];
    inputText.value = "";
    pendingImages.value = [];
    saveSessions();

    isGenerating.value = true;
    streamingContent.value = "";

    const ac = new AbortController();
    abortRef.current = ac;

    chatError.value = "";
    try {
      await streamChat(
        currentModel.model || currentModel.name,
        apiMessages,
        {
          temperature: temperature.value,
          top_p: topP.value,
          max_tokens: maxTokens.value,
          num_ctx: normalizeNumCtx(session.numCtx),
          num_parallel: session.numParallel || defaultNumParallel(),
          system: systemPrompt.value || undefined,
          source: currentModel.source,
        },
        (token, done) => {
          if (done) {
            session.messages.push({
              role: "assistant",
              content: streamingContent.value,
            });
            sessions.value = [...sessions.value];
            streamingContent.value = "";
            saveSessions();
          } else {
            streamingContent.value += token;
          }
        },
        ac.signal
      );
    } catch (e: any) {
      const errMessage = e?.message || t("chat.failedResp");
      if (streamingContent.value) {
        session.messages.push({
          role: "assistant",
          content: streamingContent.value,
        });
        sessions.value = [...sessions.value];
        streamingContent.value = "";
        saveSessions();
      } else if (!ac.signal.aborted) {
        if (currentModel.source === "cloud" && /(AUTH-ERR-1|AUTH-ERR-5|login first|Error 401)/i.test(errMessage)) {
          await openCloudAuthDialog(t("chat.cloudLoginExpired"));
        } else {
          chatError.value = errMessage;
        }
      }
    }

    isGenerating.value = false;
    abortRef.current = null;
  };

  const handleStop = () => {
    abortRef.current?.abort();
  };

  const handleNewSession = () => {
    const s: Session = { id: makeId(), title: "New Chat", messages: [], numCtx: defaultNumCtx(), numParallel: defaultNumParallel() };
    sessions.value = [s, ...sessions.value];
    activeSessionId.value = s.id;
    saveSessions();
  };

  const handleClearHistory = () => {
    const session = getActiveSession();
    if (!session || session.messages.length === 0) return;
    if (!confirm(t("chat.clearConfirm"))) return;
    session.messages = [];
    session.title = "New Chat";
    sessions.value = [...sessions.value];
    saveSessions();
  };

  const handleKeyDown = (e: KeyboardEvent) => {
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      handleSend();
    }
  };

  const session = getActiveSession();
  const messages = session?.messages || [];

  return (
    <div class="flex h-full">
      {/* Main Chat Area */}
      <div class="flex-1 flex flex-col min-w-0">
        {/* Header */}
        <div class="flex items-center justify-between px-6 py-3 border-b border-gray-200 bg-white">
          <div class="flex items-center gap-3">
            <button onClick={handleNewSession} class="text-gray-400 hover:text-gray-600" title={t("chat.newChat")}>
              <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                <path stroke-linecap="round" stroke-linejoin="round" d="M12 4v16m8-8H4" />
              </svg>
            </button>
            <span class="text-sm font-medium text-gray-700 truncate max-w-xs">{session?.title || t("chat.chat")}</span>
          </div>
          <div class="flex items-center gap-2">
            <button
              onClick={handleClearHistory}
              class="p-1.5 rounded-lg text-gray-400 hover:text-red-500 transition-colors"
              title={t("chat.clearHistory")}
            >
              <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                <path stroke-linecap="round" stroke-linejoin="round" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
              </svg>
            </button>
            <button
              onClick={() => (showSettings.value = !showSettings.value)}
              class={`p-1.5 rounded-lg transition-colors ${showSettings.value ? "bg-indigo-50 text-indigo-600" : "text-gray-400 hover:text-gray-600"}`}
              title={t("chat.settings")}
            >
              <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                <path stroke-linecap="round" stroke-linejoin="round" d="M12 6V4m0 2a2 2 0 100 4m0-4a2 2 0 110 4m-6 8a2 2 0 100-4m0 4a2 2 0 110-4m0 4v2m0-6V4m6 6v10m6-2a2 2 0 100-4m0 4a2 2 0 110-4m0 4v2m0-6V4" />
              </svg>
            </button>
          </div>
        </div>

        {/* Messages */}
        <div ref={messagesRef} class="flex-1 overflow-auto px-6 py-4 space-y-4">
          {chatError.value && (
            <div class="flex items-start gap-2 bg-red-50 border border-red-200 text-red-700 text-sm px-4 py-3 rounded-lg">
              <svg class="w-4 h-4 flex-shrink-0 mt-0.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                <path stroke-linecap="round" stroke-linejoin="round" d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
              </svg>
              <span class="whitespace-pre-line flex-1">{chatError.value}</span>
              <button onClick={() => (chatError.value = "")} class="ml-auto text-red-400 hover:text-red-600 flex-shrink-0">&#x2715;</button>
            </div>
          )}
          {messages.length === 0 && !streamingContent.value && !chatError.value && (
            <div class="text-center text-gray-400 text-sm mt-20">{t("chat.startConv")}</div>
          )}
          {messages.map((m, i) => (
            <MessageBubble key={i} message={m} />
          ))}
          {streamingContent.value && (
            <MessageBubble message={{ role: "assistant", content: streamingContent.value }} streaming />
          )}
        </div>

        {/* Input */}
        <div class="px-6 py-4 border-t border-gray-200 bg-white">
          {/* Image previews */}
          {pendingImages.value.length > 0 && (
            <div class="flex gap-2 mb-2 flex-wrap">
              {pendingImages.value.map((img, i) => (
                <div key={i} class="relative group">
                  <img src={img.thumb} class="w-16 h-16 object-cover rounded-lg border border-gray-200" />
                  <button
                    onClick={() => removeImage(i)}
                    class="absolute -top-1.5 -right-1.5 w-5 h-5 bg-red-500 text-white rounded-full text-xs flex items-center justify-center opacity-0 group-hover:opacity-100 transition-opacity"
                  >
                    x
                  </button>
                </div>
              ))}
            </div>
          )}
          <div class="flex items-center gap-3">
            {/* Model selector */}
            <select
              class="border border-gray-200 rounded-lg px-3 py-2 text-sm text-gray-600 focus:outline-none focus:ring-2 focus:ring-indigo-500 max-w-[200px]"
              value={selectedModelKey.value}
              onChange={(e) => handleModelChange((e.target as HTMLSelectElement).value)}
            >
              {availableModels.value.map((m) => (
                <option key={modelKey(m)} value={modelKey(m)}>
                  {modelLabel(m)}
                </option>
              ))}
            </select>
            {/* Image upload for vision models */}
            {isVisionModel.value && (
              <label class="p-2.5 rounded-lg border border-gray-200 text-gray-500 hover:text-indigo-600 hover:border-indigo-300 cursor-pointer transition-colors" title={t("chat.uploadImage")}>
                <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M4 16l4.586-4.586a2 2 0 012.828 0L16 16m-2-2l1.586-1.586a2 2 0 012.828 0L20 14m-6-6h.01M6 20h12a2 2 0 002-2V6a2 2 0 00-2-2H6a2 2 0 00-2 2v12a2 2 0 002 2z" />
                </svg>
                <input type="file" accept="image/*" multiple class="hidden" onChange={handleImageUpload} />
              </label>
            )}
            <div class="flex-1 relative">
              <textarea
                class="w-full border border-gray-200 rounded-lg px-4 py-2.5 text-sm resize-none focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:border-transparent"
                rows={1}
                placeholder={isVisionModel.value ? t("chat.askImage") : t("chat.askHelp")}
                value={inputText.value}
                onInput={(e) => (inputText.value = (e.target as HTMLTextAreaElement).value)}
                onKeyDown={handleKeyDown}
              />
            </div>
            {isGenerating.value ? (
              <button
                onClick={handleStop}
                class="p-2.5 rounded-lg bg-red-500 text-white hover:bg-red-600 transition-colors"
                title={t("chat.stop")}
              >
                <svg class="w-5 h-5" fill="currentColor" viewBox="0 0 24 24">
                  <rect x="6" y="6" width="12" height="12" rx="1" />
                </svg>
              </button>
            ) : (
              <button
                onClick={handleSend}
                disabled={!inputText.value.trim() || !selectedModelInfo.value}
                class="p-2.5 rounded-lg bg-indigo-600 text-white hover:bg-indigo-700 disabled:opacity-40 transition-colors"
                title={t("chat.send")}
              >
                <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M5 12h14m-7-7l7 7-7 7" />
                </svg>
              </button>
            )}
          </div>
        </div>
      </div>

      {/* Settings Panel */}
      {showSettings.value && (
        <div class="w-72 border-l border-gray-200 bg-white p-5 flex flex-col gap-5 overflow-auto">
          <div class="flex items-center justify-between">
            <h3 class="font-semibold text-gray-900">{t("chat.suggestion")}</h3>
            <button onClick={() => (showSettings.value = false)} class="text-gray-400 hover:text-gray-600">
              <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                <path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" />
              </svg>
            </button>
          </div>

          <div>
            <label class="block text-sm font-medium text-gray-700 mb-1">{t("chat.systemPrompt")}</label>
            <textarea
              class="w-full border border-gray-200 rounded-lg px-3 py-2 text-sm resize-none focus:outline-none focus:ring-2 focus:ring-indigo-500"
              rows={3}
              placeholder={t("chat.placeholder")}
              value={systemPrompt.value}
              onInput={(e) => (systemPrompt.value = (e.target as HTMLTextAreaElement).value)}
            />
          </div>

          <SliderSetting label="max_tokens" value={maxTokens.value} min={1} max={8192} step={1} onChange={(v) => (maxTokens.value = v)} />
          <div>
            <div class="flex items-center justify-between mb-1.5">
              <label class="text-sm font-medium text-gray-700">num_ctx</label>
              <span class="text-sm text-gray-500 tabular-nums">{normalizeNumCtx(session?.numCtx)}</span>
            </div>
            <input
              type="range"
              min={0}
              max={contextLengthSteps.length - 1}
              step={1}
              value={Math.max(0, contextLengthSteps.indexOf(normalizeNumCtx(session?.numCtx)))}
              onInput={(e) => {
                const idx = Number((e.target as HTMLInputElement).value);
                const s = getActiveSession();
                if (!s) return;
                s.numCtx = contextLengthSteps[idx] || defaultNumCtx();
                sessions.value = [...sessions.value];
                saveSessions();
              }}
              class="w-full h-1.5 bg-gray-200 rounded-lg appearance-none cursor-pointer accent-indigo-600"
            />
            <div class="flex justify-between mt-1.5">
              {contextLengthLabels.map((label) => (
                <span key={label} class="text-[10px] text-gray-400">{label}</span>
              ))}
            </div>
          </div>
          <SliderSetting label="Temperature" value={temperature.value} min={0} max={2} step={0.05} onChange={(v) => (temperature.value = v)} />
          <SliderSetting label="Top-P" value={topP.value} min={0} max={1} step={0.05} onChange={(v) => (topP.value = v)} />

          <button
            onClick={() => {
              systemPrompt.value = "";
              temperature.value = 0.95;
              topP.value = 0.75;
              maxTokens.value = 4096;
            }}
            class="flex items-center justify-center gap-1.5 py-2 border border-gray-200 rounded-lg text-sm text-gray-600 hover:bg-gray-50 transition-colors"
          >
            <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
              <path stroke-linecap="round" stroke-linejoin="round" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
            </svg>
            {t("chat.resetDefaults")}
          </button>
        </div>
      )}
      {showCloudAuthDialog.value && (
        <div class="fixed inset-0 z-50 flex items-center justify-center bg-gray-900/40 px-4">
          <div class="w-full max-w-lg rounded-2xl bg-white p-6 shadow-2xl">
            {/* Model name display with copy button */}
            {selectedModelInfo.value && (
              <div class="mb-4 flex items-center justify-between rounded-lg bg-indigo-50 px-3 py-2">
                <div class="flex items-center gap-2">
                  <svg class="w-4 h-4 text-indigo-600" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                    <path stroke-linecap="round" stroke-linejoin="round" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
                  </svg>
                  <span class="text-sm font-medium text-indigo-700">{modelLabel(selectedModelInfo.value)}</span>
                </div>
                <button
                  onClick={() => {
                    const modelName = selectedModelInfo.value?.model || selectedModelInfo.value?.name || "";
                    navigator.clipboard.writeText(modelName);
                  }}
                  class="rounded p-1 text-indigo-600 hover:bg-indigo-100 transition-colors"
                  title={t("chat.copyModel")}
                >
                  <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                    <path stroke-linecap="round" stroke-linejoin="round" d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z" />
                  </svg>
                </button>
              </div>
            )}
            <div class="flex items-start justify-between gap-4">
              <div>
                <h3 class="text-lg font-semibold text-gray-900">{t("chat.cloudLoginTitle")}</h3>
                <p class="mt-2 text-sm leading-6 text-gray-500">{t("chat.cloudLoginDesc")}</p>
              </div>
              <button
                onClick={() => {
                  showCloudAuthDialog.value = false;
                  cloudAuthError.value = "";
                }}
                class="rounded-lg p-1 text-gray-400 hover:text-gray-600"
                aria-label={t("chat.cloudCancel")}
              >
                <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" />
                </svg>
              </button>
            </div>

            {cloudAuthError.value && (
              <div class="mt-4 rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">
                {cloudAuthError.value}
              </div>
            )}

            <div class="mt-5 flex flex-wrap gap-2">
              <button
                onClick={handleOpenCloudLogin}
                class="rounded-lg border border-gray-200 px-4 py-2 text-sm text-gray-700 hover:bg-gray-50 transition-colors"
              >
                {t("chat.cloudOpenLogin")}
              </button>
              <button
                onClick={handleOpenCloudTokenPage}
                class="rounded-lg border border-gray-200 px-4 py-2 text-sm text-gray-700 hover:bg-gray-50 transition-colors"
              >
                {t("chat.cloudOpenTokenPage")}
              </button>
            </div>

            <div class="mt-5">
              <label class="mb-2 block text-sm font-medium text-gray-700">{t("chat.cloudTokenLabel")}</label>
              <input
                type="password"
                autoComplete="off"
                spellcheck={false}
                class="w-full rounded-lg border border-gray-200 px-4 py-2.5 text-sm focus:outline-none focus:ring-2 focus:ring-indigo-500"
                placeholder={t("chat.cloudTokenPlaceholder")}
                value={cloudTokenInput.value}
                onInput={(e) => (cloudTokenInput.value = (e.target as HTMLInputElement).value)}
              />
            </div>

            <div class="mt-5 flex justify-end gap-2">
              <button
                onClick={() => {
                  showCloudAuthDialog.value = false;
                  cloudAuthError.value = "";
                }}
                class="rounded-lg border border-gray-200 px-4 py-2 text-sm text-gray-700 hover:bg-gray-50 transition-colors"
              >
                {t("chat.cloudCancel")}
              </button>
              <button
                onClick={handleSaveCloudToken}
                disabled={isSavingCloudToken.value}
                class="rounded-lg bg-indigo-600 px-4 py-2 text-sm text-white hover:bg-indigo-700 disabled:opacity-60 transition-colors"
              >
                {isSavingCloudToken.value ? t("chat.cloudSavingToken") : t("chat.cloudSaveToken")}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

function MessageBubble({ message, streaming }: { message: ChatMessage; streaming?: boolean }) {
  const isUser = message.role === "user";
  const content = message.content;

  const renderContent = () => {
    if (typeof content === "string") {
      if (!isUser) {
        const parsed = parseReasoningText(content);
        if (parsed.hasThinking) {
          const thinkingLabel = parsed.thinkingOpen ? t("chat.thinkingLive") : t("chat.thinking");
          return (
            <>
              <details
                open={streaming || parsed.thinkingOpen}
                class={`rounded-xl border border-amber-200 bg-amber-50/70 px-3 py-2 ${
                  parsed.answer ? "mb-3" : ""
                }`}
              >
                <summary class="cursor-pointer select-none text-xs font-medium text-amber-700">
                  {thinkingLabel}
                </summary>
                {parsed.thinking && (
                  <div class="mt-2 whitespace-pre-wrap text-xs leading-relaxed text-amber-900">
                    {parsed.thinking}
                  </div>
                )}
              </details>
              {parsed.answer && <p class="whitespace-pre-wrap">{parsed.answer}</p>}
            </>
          );
        }
      }
      return <p class="whitespace-pre-wrap">{content}</p>;
    }
    if (Array.isArray(content)) {
      return (
        <>
          {(content as ContentPart[]).map((part, i) => {
            if (part.type === "image_url" && part.image_url) {
              return <img key={i} src={part.image_url.url} class="max-w-full rounded-lg mb-2 max-h-64" />;
            }
            if (part.type === "text" && part.text) {
              return <p key={i} class="whitespace-pre-wrap">{part.text}</p>;
            }
            return null;
          })}
        </>
      );
    }
    return null;
  };

  return (
    <div class={`flex ${isUser ? "justify-end" : "justify-start"}`}>
      <div
        class={`max-w-[75%] rounded-2xl px-4 py-3 text-sm leading-relaxed ${
          isUser
            ? "bg-indigo-600 text-white"
            : "bg-white border border-gray-200 text-gray-800"
        } ${streaming ? "animate-pulse" : ""}`}
      >
        {renderContent()}
      </div>
    </div>
  );
}

function SliderSetting({
  label,
  value,
  min,
  max,
  step,
  onChange,
}: {
  label: string;
  value: number;
  min: number;
  max: number;
  step: number;
  onChange: (v: number) => void;
}) {
  return (
    <div>
      <div class="flex items-center justify-between mb-1.5">
        <label class="text-sm font-medium text-gray-700">{label}</label>
        <span class="text-sm text-gray-500 tabular-nums">{value}</span>
      </div>
      <input
        type="range"
        min={min}
        max={max}
        step={step}
        value={value}
        onInput={(e) => onChange(parseFloat((e.target as HTMLInputElement).value))}
        class="w-full h-1.5 bg-gray-200 rounded-lg appearance-none cursor-pointer accent-indigo-600"
      />
    </div>
  );
}
