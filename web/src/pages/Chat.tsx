import { useEffect, useRef } from "preact/hooks";
import { signal } from "@preact/signals";
import { getTags, getPs, streamChat } from "../api/client";
import type { ModelInfo, ChatMessage } from "../api/client";

interface Session {
  id: string;
  title: string;
  messages: ChatMessage[];
}

const availableModels = signal<ModelInfo[]>([]);
const selectedModel = signal("");
const sessions = signal<Session[]>(loadSessions());
const activeSessionId = signal(sessions.value[0]?.id || "");
const inputText = signal("");
const isGenerating = signal(false);
const showSettings = signal(false);

const systemPrompt = signal("");
const temperature = signal(0.95);
const topP = signal(0.75);
const maxTokens = signal(4096);

const streamingContent = signal("");

function loadSessions(): Session[] {
  try {
    const raw = localStorage.getItem("csghub-chat-sessions");
    if (raw) return JSON.parse(raw);
  } catch { /* ignore */ }
  const s: Session = { id: crypto.randomUUID(), title: "New Chat", messages: [] };
  return [s];
}

function saveSessions() {
  localStorage.setItem("csghub-chat-sessions", JSON.stringify(sessions.value));
}

function getActiveSession(): Session | undefined {
  return sessions.value.find((s) => s.id === activeSessionId.value);
}

export function Chat() {
  const messagesRef = useRef<HTMLDivElement>(null);
  const abortRef = useRef<AbortController | null>(null);

  useEffect(() => {
    getTags().then((m) => {
      availableModels.value = m;
      if (!selectedModel.value && m.length > 0) {
        selectedModel.value = m[0].name;
      }
    }).catch(() => {});
    getPs().then((running) => {
      if (running.length > 0 && !selectedModel.value) {
        selectedModel.value = running[0].name;
      }
    }).catch(() => {});
  }, []);

  useEffect(() => {
    if (messagesRef.current) {
      messagesRef.current.scrollTop = messagesRef.current.scrollHeight;
    }
  }, [getActiveSession()?.messages.length, streamingContent.value]);

  const handleSend = async () => {
    const text = inputText.value.trim();
    if (!text || !selectedModel.value || isGenerating.value) return;

    const session = getActiveSession();
    if (!session) return;

    session.messages.push({ role: "user", content: text });
    if (session.messages.length === 1) {
      session.title = text.slice(0, 30) || "New Chat";
    }
    sessions.value = [...sessions.value];
    inputText.value = "";
    saveSessions();

    isGenerating.value = true;
    streamingContent.value = "";

    const ac = new AbortController();
    abortRef.current = ac;

    const startTime = Date.now();

    try {
      await streamChat(
        selectedModel.value,
        session.messages,
        {
          temperature: temperature.value,
          top_p: topP.value,
          max_tokens: maxTokens.value,
          system: systemPrompt.value || undefined,
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
    } catch {
      if (streamingContent.value) {
        session.messages.push({
          role: "assistant",
          content: streamingContent.value,
        });
        sessions.value = [...sessions.value];
        streamingContent.value = "";
        saveSessions();
      }
    }

    const elapsed = ((Date.now() - startTime) / 1000).toFixed(1);
    void elapsed;
    isGenerating.value = false;
    abortRef.current = null;
  };

  const handleStop = () => {
    abortRef.current?.abort();
  };

  const handleNewSession = () => {
    const s: Session = { id: crypto.randomUUID(), title: "New Chat", messages: [] };
    sessions.value = [s, ...sessions.value];
    activeSessionId.value = s.id;
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
            <button onClick={handleNewSession} class="text-gray-400 hover:text-gray-600" title="New Chat">
              <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                <path stroke-linecap="round" stroke-linejoin="round" d="M12 4v16m8-8H4" />
              </svg>
            </button>
            <span class="text-sm font-medium text-gray-700 truncate max-w-xs">{session?.title || "Chat"}</span>
          </div>
          <button
            onClick={() => (showSettings.value = !showSettings.value)}
            class={`p-1.5 rounded-lg transition-colors ${showSettings.value ? "bg-indigo-50 text-indigo-600" : "text-gray-400 hover:text-gray-600"}`}
            title="Settings"
          >
            <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
              <path stroke-linecap="round" stroke-linejoin="round" d="M12 6V4m0 2a2 2 0 100 4m0-4a2 2 0 110 4m-6 8a2 2 0 100-4m0 4a2 2 0 110-4m0 4v2m0-6V4m6 6v10m6-2a2 2 0 100-4m0 4a2 2 0 110-4m0 4v2m0-6V4" />
            </svg>
          </button>
        </div>

        {/* Messages */}
        <div ref={messagesRef} class="flex-1 overflow-auto px-6 py-4 space-y-4">
          {messages.length === 0 && !streamingContent.value && (
            <div class="text-center text-gray-400 text-sm mt-20">Start a conversation...</div>
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
          <div class="flex items-center gap-3">
            {/* Model selector */}
            <select
              class="border border-gray-200 rounded-lg px-3 py-2 text-sm text-gray-600 focus:outline-none focus:ring-2 focus:ring-indigo-500 max-w-[200px]"
              value={selectedModel.value}
              onChange={(e) => (selectedModel.value = (e.target as HTMLSelectElement).value)}
            >
              {availableModels.value.map((m) => (
                <option key={m.name} value={m.name}>{m.name}</option>
              ))}
            </select>
            <div class="flex-1 relative">
              <textarea
                class="w-full border border-gray-200 rounded-lg px-4 py-2.5 text-sm resize-none focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:border-transparent"
                rows={1}
                placeholder="How can I help you?"
                value={inputText.value}
                onInput={(e) => (inputText.value = (e.target as HTMLTextAreaElement).value)}
                onKeyDown={handleKeyDown}
              />
            </div>
            {isGenerating.value ? (
              <button
                onClick={handleStop}
                class="p-2.5 rounded-lg bg-red-500 text-white hover:bg-red-600 transition-colors"
                title="Stop"
              >
                <svg class="w-5 h-5" fill="currentColor" viewBox="0 0 24 24">
                  <rect x="6" y="6" width="12" height="12" rx="1" />
                </svg>
              </button>
            ) : (
              <button
                onClick={handleSend}
                disabled={!inputText.value.trim() || !selectedModel.value}
                class="p-2.5 rounded-lg bg-indigo-600 text-white hover:bg-indigo-700 disabled:opacity-40 transition-colors"
                title="Send"
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
            <h3 class="font-semibold text-gray-900">Make a Suggestion</h3>
            <button onClick={() => (showSettings.value = false)} class="text-gray-400 hover:text-gray-600">
              <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                <path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" />
              </svg>
            </button>
          </div>

          <div>
            <label class="block text-sm font-medium text-gray-700 mb-1">System Prompt</label>
            <textarea
              class="w-full border border-gray-200 rounded-lg px-3 py-2 text-sm resize-none focus:outline-none focus:ring-2 focus:ring-indigo-500"
              rows={3}
              placeholder="Placeholder text..."
              value={systemPrompt.value}
              onInput={(e) => (systemPrompt.value = (e.target as HTMLTextAreaElement).value)}
            />
          </div>

          <SliderSetting label="max_tokens" value={maxTokens.value} min={1} max={8192} step={1} onChange={(v) => (maxTokens.value = v)} />
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
            Reset Defaults
          </button>
        </div>
      )}
    </div>
  );
}

function MessageBubble({ message, streaming }: { message: ChatMessage; streaming?: boolean }) {
  const isUser = message.role === "user";
  return (
    <div class={`flex ${isUser ? "justify-end" : "justify-start"}`}>
      <div
        class={`max-w-[75%] rounded-2xl px-4 py-3 text-sm leading-relaxed ${
          isUser
            ? "bg-indigo-600 text-white"
            : "bg-white border border-gray-200 text-gray-800"
        } ${streaming ? "animate-pulse" : ""}`}
      >
        <p class="whitespace-pre-wrap">{message.content}</p>
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
