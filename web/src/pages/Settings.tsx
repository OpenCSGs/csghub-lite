import { signal } from "@preact/signals";
import { useEffect } from "preact/hooks";
import { t, locale, setLocale } from "../i18n";
import type { Locale } from "../i18n";
import { clearCloudToken, getCloudAuthStatus } from "../api/client";
import type { CloudAuthStatus } from "../api/client";

const contextLengthSteps = [4096, 8192, 16384, 32768, 65536, 131072, 262144];
const contextLengthLabels = ["4k", "8k", "16k", "32k", "64k", "128k", "256k"];
const contextStorageKey = "csghub.chat.num_ctx";

const modelLocation = signal("");
const appVersion = signal("");
const contextIndex = signal(1);
const cloudAuth = signal<CloudAuthStatus | null>(null);
const cloudAuthError = signal("");
const isClearingCloudToken = signal(false);

function loadContextIndex(): number {
  try {
    const raw = localStorage.getItem(contextStorageKey);
    const num = Number(raw);
    const idx = contextLengthSteps.indexOf(num);
    if (idx >= 0) return idx;
  } catch {
    /* ignore */
  }
  return 1;
}

function saveContextIndex(idx: number) {
  const value = contextLengthSteps[idx] || contextLengthSteps[1];
  try {
    localStorage.setItem(contextStorageKey, String(value));
  } catch {
    /* ignore */
  }
}

function resetDefaults() {
  contextIndex.value = 1;
  saveContextIndex(1);
  fetchSettings();
}

function fetchSettings() {
  fetch("/api/settings")
    .then((r) => r.json())
    .then((data: { version: string; model_dir: string }) => {
      modelLocation.value = data.model_dir || "";
      appVersion.value = data.version || "";
    })
    .catch(() => {});
}

function fetchCloudAuth() {
  getCloudAuthStatus()
    .then((status) => {
      cloudAuth.value = status;
      cloudAuthError.value = "";
    })
    .catch((err: any) => {
      cloudAuth.value = null;
      cloudAuthError.value = err?.message || "";
    });
}

function openExternal(url?: string) {
  if (url) {
    window.open(url, "_blank", "noopener,noreferrer");
  }
}

function cloudUserLabel(status: CloudAuthStatus | null): string {
  const user = status?.user;
  return (user?.nickname || user?.username || "").trim();
}

function cloudUserInitial(status: CloudAuthStatus | null): string {
  const label = cloudUserLabel(status);
  return label ? label[0].toUpperCase() : "?";
}

export function Settings() {
  void locale.value;

  useEffect(() => {
    fetchSettings();
    fetchCloudAuth();
    contextIndex.value = loadContextIndex();
  }, []);

  const handleOpenCloudLogin = () => {
    openExternal(cloudAuth.value?.login_url);
  };

  const handleOpenCloudTokenPage = () => {
    openExternal(cloudAuth.value?.access_token_url);
  };

  const handleLogout = async () => {
    if (isClearingCloudToken.value) return;
    isClearingCloudToken.value = true;
    cloudAuthError.value = "";
    try {
      cloudAuth.value = await clearCloudToken();
    } catch (err: any) {
      cloudAuthError.value = err?.message || t("chat.failedResp");
    } finally {
      isClearingCloudToken.value = false;
    }
  };

  return (
    <div class="p-8 max-w-3xl mx-auto">
      <h1 class="text-2xl font-bold text-gray-900">{t("settings.title")}</h1>
      <p class="text-gray-500 text-sm mt-1 mb-10">{t("settings.subtitle")}</p>

      {/* Model location */}
      <div class="mb-10">
        <div class="flex items-center gap-2 mb-1">
          <svg class="w-5 h-5 text-gray-600" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
            <path stroke-linecap="round" stroke-linejoin="round" d="M3 7v10a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-6l-2-2H5a2 2 0 00-2 2z" />
          </svg>
          <span class="font-semibold text-gray-900">{t("settings.modelLocation")}</span>
        </div>
        <p class="text-sm text-gray-500 mb-3 ml-7">{t("settings.modelLocationDesc")}</p>
        <div class="ml-7">
          <span class="inline-block px-4 py-2.5 border border-gray-200 rounded-lg text-sm text-gray-500 bg-gray-50 select-all">
            {modelLocation.value || "..."}
          </span>
        </div>
      </div>

      {/* Context length */}
      <div class="mb-10">
        <div class="flex items-center gap-2 mb-1">
          <svg class="w-5 h-5 text-gray-600" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
            <path stroke-linecap="round" stroke-linejoin="round" d="M13 10V3L4 14h7v7l9-11h-7z" />
          </svg>
          <span class="font-semibold text-gray-900">{t("settings.contextLength")}</span>
        </div>
        <p class="text-sm text-gray-500 mb-4 ml-7">{t("settings.contextLengthDesc")}</p>
        <div class="ml-7">
          <input
            type="range"
            min="0"
            max={contextLengthSteps.length - 1}
            step="1"
            value={contextIndex.value}
            onInput={(e) => {
              const idx = Number((e.target as HTMLInputElement).value);
              contextIndex.value = idx;
              saveContextIndex(idx);
            }}
            class="w-full h-1.5 bg-gray-200 rounded-full appearance-none cursor-pointer accent-indigo-600"
          />
          <div class="flex justify-between mt-2">
            {contextLengthLabels.map((label) => (
              <span key={label} class="text-xs text-gray-400">{label}</span>
            ))}
          </div>
        </div>
      </div>

      {/* Language */}
      <div class="mb-10">
        <div class="flex items-center gap-2 mb-1">
          <svg class="w-5 h-5 text-gray-600" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
            <path stroke-linecap="round" stroke-linejoin="round" d="M3 5h12M9 3v2m1.048 9.5A18.022 18.022 0 016.412 9m6.088 9h7M11 21l5-10 5 10M12.751 5C11.783 10.77 8.07 15.61 3 18.129" />
          </svg>
          <span class="font-semibold text-gray-900">{t("settings.language")}</span>
        </div>
        <p class="text-sm text-gray-500 mb-3 ml-7">{t("settings.languageDesc")}</p>
        <div class="flex gap-2 ml-7">
          <LangBtn code="en" label="EN" />
          <LangBtn code="zh" label="中文" />
        </div>
      </div>

      {/* Account */}
      <div class="mb-10">
        <div class="flex items-center gap-2 mb-1">
          <svg class="w-5 h-5 text-gray-600" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
            <path stroke-linecap="round" stroke-linejoin="round" d="M5.121 17.804A9 9 0 1118.88 17.8M15 11a3 3 0 11-6 0 3 3 0 016 0z" />
          </svg>
          <span class="font-semibold text-gray-900">{t("settings.account")}</span>
        </div>
        <p class="text-sm text-gray-500 mb-3 ml-7">{t("settings.accountDesc")}</p>
        <div class="ml-7 rounded-xl border border-gray-200 bg-white p-4">
          {cloudAuth.value === null ? (
            <p class="text-sm text-gray-500">...</p>
          ) : cloudAuth.value.authenticated && cloudAuth.value.user ? (
            <div class="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
              <div class="flex items-center gap-4 min-w-0">
                {cloudAuth.value.user.avatar ? (
                  <img
                    src={cloudAuth.value.user.avatar}
                    alt={cloudUserLabel(cloudAuth.value)}
                    class="w-12 h-12 rounded-full border border-gray-200 object-cover bg-gray-50"
                  />
                ) : (
                  <div class="w-12 h-12 rounded-full bg-indigo-50 text-indigo-700 flex items-center justify-center text-lg font-semibold">
                    {cloudUserInitial(cloudAuth.value)}
                  </div>
                )}
                <div class="min-w-0">
                  <p class="text-sm font-semibold text-gray-900 truncate">{cloudUserLabel(cloudAuth.value)}</p>
                  <p class="text-sm text-gray-500 truncate">@{cloudAuth.value.user.username}</p>
                  {cloudAuth.value.user.email && (
                    <p class="text-sm text-gray-500 truncate">{cloudAuth.value.user.email}</p>
                  )}
                </div>
              </div>
              <div class="flex gap-2">
                <button
                  onClick={handleOpenCloudTokenPage}
                  class="px-4 py-2 border border-gray-200 rounded-lg text-sm text-gray-700 hover:bg-gray-50 transition-colors"
                >
                  {t("settings.openTokenPage")}
                </button>
                <button
                  onClick={handleLogout}
                  disabled={isClearingCloudToken.value}
                  class="px-4 py-2 border border-red-200 rounded-lg text-sm text-red-600 hover:bg-red-50 disabled:opacity-60 disabled:cursor-not-allowed transition-colors"
                >
                  {isClearingCloudToken.value ? t("settings.loggingOut") : t("settings.logout")}
                </button>
              </div>
            </div>
          ) : cloudAuth.value.has_token ? (
            <div class="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
              <div>
                <p class="text-sm font-semibold text-gray-900">{t("settings.tokenSaved")}</p>
                <p class="text-sm text-gray-500">{t("settings.tokenSavedDesc")}</p>
              </div>
              <div class="flex gap-2">
                <button
                  onClick={handleOpenCloudLogin}
                  class="px-4 py-2 border border-gray-200 rounded-lg text-sm text-gray-700 hover:bg-gray-50 transition-colors"
                >
                  {t("settings.login")}
                </button>
                <button
                  onClick={handleLogout}
                  disabled={isClearingCloudToken.value}
                  class="px-4 py-2 border border-red-200 rounded-lg text-sm text-red-600 hover:bg-red-50 disabled:opacity-60 disabled:cursor-not-allowed transition-colors"
                >
                  {isClearingCloudToken.value ? t("settings.loggingOut") : t("settings.logout")}
                </button>
              </div>
            </div>
          ) : (
            <div class="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
              <div>
                <p class="text-sm font-semibold text-gray-900">{t("settings.loggedOut")}</p>
                <p class="text-sm text-gray-500">{t("settings.loggedOutDesc")}</p>
              </div>
              <div class="flex gap-2">
                <button
                  onClick={handleOpenCloudLogin}
                  class="px-4 py-2 border border-gray-200 rounded-lg text-sm text-gray-700 hover:bg-gray-50 transition-colors"
                >
                  {t("settings.login")}
                </button>
                <button
                  onClick={handleOpenCloudTokenPage}
                  class="px-4 py-2 border border-gray-200 rounded-lg text-sm text-gray-700 hover:bg-gray-50 transition-colors"
                >
                  {t("settings.openTokenPage")}
                </button>
              </div>
            </div>
          )}
          {cloudAuthError.value && (
            <p class="mt-3 text-sm text-red-600">{cloudAuthError.value}</p>
          )}
        </div>
      </div>

      {/* Version information */}
      <div class="flex items-center justify-between">
        <div>
          <div class="flex items-center gap-2 mb-1">
            <svg class="w-5 h-5 text-gray-600" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
              <path stroke-linecap="round" stroke-linejoin="round" d="M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z" />
            </svg>
            <span class="font-semibold text-gray-900">{t("settings.versionInfo")}</span>
          </div>
          <p class="text-sm text-gray-500 ml-7">{appVersion.value || "..."}</p>
        </div>
        <button
          onClick={resetDefaults}
          class="px-4 py-2 border border-gray-200 rounded-lg text-sm text-gray-700 hover:bg-gray-50 transition-colors"
        >
          {t("settings.resetDefaults")}
        </button>
      </div>
    </div>
  );
}

function LangBtn({ code, label }: { code: Locale; label: string }) {
  const active = locale.value === code;
  return (
    <button
      onClick={() => setLocale(code)}
      class={`px-4 py-2 text-sm rounded-lg border transition-colors ${
        active
          ? "bg-indigo-50 border-indigo-300 text-indigo-700 font-medium"
          : "border-gray-200 text-gray-600 hover:bg-gray-50"
      }`}
    >
      {label}
    </button>
  );
}
