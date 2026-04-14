import { signal } from "@preact/signals";
import { useEffect } from "preact/hooks";
import { DirectoryPickerDialog } from "../components/DirectoryPickerDialog";
import { t, locale, setLocale } from "../i18n";
import type { Locale } from "../i18n";
import {
  browseLocalDirectories,
  clearCloudToken,
  getCloudAuthStatus,
  getSettings,
  saveCloudToken,
  saveSettings,
} from "../api/client";
import type { AppSettings, CloudAuthStatus, LocalDirectoryBrowseResponse } from "../api/client";

const contextLengthSteps = [4096, 8192, 16384, 32768, 65536, 131072, 262144];
const contextLengthLabels = ["4k", "8k", "16k", "32k", "64k", "128k", "256k"];
const contextStorageKey = "csghub.chat.num_ctx";
const parallelSteps = [1, 2, 4, 8];
const parallelLabels = ["1", "2", "4", "8"];
const parallelStorageKey = "csghub.chat.num_parallel";

const storageLocation = signal("");
const modelDirectory = signal("");
const datasetDirectory = signal("");
const appVersion = signal("");
const contextIndex = signal(1);
const parallelIndex = signal(2);
const cloudAuth = signal<CloudAuthStatus | null>(null);
const cloudTokenInput = signal("");
const cloudAuthError = signal("");
const isClearingCloudToken = signal(false);
const isSavingCloudToken = signal(false);
const isSavingStorageDir = signal(false);
const storageDirInput = signal("");
const storageDirError = signal("");
const isBrowsingStorageDir = signal(false);
const isStorageDirPickerOpen = signal(false);
const storageDirBrowser = signal<LocalDirectoryBrowseResponse | null>(null);
const storageDirBrowserError = signal("");

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

function loadParallelIndex(): number {
  try {
    const raw = localStorage.getItem(parallelStorageKey);
    const num = Number(raw);
    const idx = parallelSteps.indexOf(num);
    if (idx >= 0) return idx;
  } catch {
    /* ignore */
  }
  return 2; // default index for 4
}

function saveParallelIndex(idx: number) {
  const value = parallelSteps[idx] || parallelSteps[2];
  try {
    localStorage.setItem(parallelStorageKey, String(value));
  } catch {
    /* ignore */
  }
}

function resetDefaults() {
  contextIndex.value = 1;
  saveContextIndex(1);
  parallelIndex.value = 2;
  saveParallelIndex(2);
  fetchSettings();
}

function applySettings(data: AppSettings) {
  storageLocation.value = data.storage_dir || "";
  storageDirInput.value = data.storage_dir || "";
  modelDirectory.value = data.model_dir || "";
  datasetDirectory.value = data.dataset_dir || "";
  appVersion.value = data.version || "";
}

function fetchSettings() {
  getSettings()
    .then((data) => {
      applySettings(data);
      storageDirError.value = "";
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

async function saveStorageDir() {
  const newDir = storageDirInput.value.trim();
  if (!newDir) return;

  isSavingStorageDir.value = true;
  storageDirError.value = "";
  try {
    const data = await saveSettings({ storage_dir: newDir });
    applySettings(data);
  } catch (err: any) {
    storageDirError.value = err?.message || t("settings.storageDirSaveFailed");
  } finally {
    isSavingStorageDir.value = false;
  }
}

async function browseStorageDir(path?: string) {
  isBrowsingStorageDir.value = true;
  storageDirBrowserError.value = "";
  try {
    storageDirBrowser.value = await browseLocalDirectories(path);
  } catch (err: any) {
    storageDirBrowserError.value = err?.message || t("settings.directoryBrowseFailed");
  } finally {
    isBrowsingStorageDir.value = false;
  }
}

function openStorageDirPicker() {
  isStorageDirPickerOpen.value = true;
  void browseStorageDir(storageLocation.value || storageDirInput.value);
}

function closeStorageDirPicker() {
  isStorageDirPickerOpen.value = false;
  storageDirBrowserError.value = "";
}

function selectStorageDir(path: string) {
  storageDirInput.value = path;
  storageDirError.value = "";
  closeStorageDirPicker();
}

function cloudUserLabel(status: CloudAuthStatus | null): string {
  const user = status?.user;
  return (user?.nickname || user?.username || "").trim();
}

function cloudUserInitial(status: CloudAuthStatus | null): string {
  const label = cloudUserLabel(status);
  return label ? label[0].toUpperCase() : "?";
}

function hasCloudAuth(status: CloudAuthStatus | null | undefined): boolean {
  return status?.authenticated ?? status?.has_token ?? false;
}

export function Settings() {
  void locale.value;
  const showTokenInput = !(cloudAuth.value?.authenticated && cloudAuth.value?.user);

  useEffect(() => {
    fetchSettings();
    fetchCloudAuth();
    contextIndex.value = loadContextIndex();
    parallelIndex.value = loadParallelIndex();
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
      cloudTokenInput.value = "";
    } catch (err: any) {
      cloudAuthError.value = err?.message || t("chat.failedResp");
    } finally {
      isSavingCloudToken.value = false;
    }
  };

  return (
    <div class="p-8 max-w-3xl mx-auto">
      <h1 class="text-2xl font-bold text-gray-900">{t("settings.title")}</h1>
      <p class="text-gray-500 text-sm mt-1 mb-10">{t("settings.subtitle")}</p>

      {/* Storage location */}
      <div class="mb-10">
        <div class="flex items-center gap-2 mb-1">
          <svg class="w-5 h-5 text-gray-600" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
            <path stroke-linecap="round" stroke-linejoin="round" d="M3 7v10a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-6l-2-2H5a2 2 0 00-2 2z" />
          </svg>
          <span class="font-semibold text-gray-900">{t("settings.modelLocation")}</span>
        </div>
        <p class="text-sm text-gray-500 mb-3 ml-7">{t("settings.modelLocationDesc")}</p>
        <div class="ml-7 flex flex-col sm:flex-row gap-3">
          <input
            type="text"
            spellcheck={false}
            class="flex-1 rounded-lg border border-gray-200 px-4 py-2.5 text-sm focus:outline-none focus:ring-2 focus:ring-indigo-500"
            value={storageDirInput.value}
            onInput={(e) => (storageDirInput.value = (e.target as HTMLInputElement).value)}
          />
          <button
            onClick={openStorageDirPicker}
            disabled={isBrowsingStorageDir.value}
            class="px-4 py-2 border border-gray-200 rounded-lg text-sm text-gray-700 hover:bg-gray-50 disabled:opacity-60 disabled:cursor-not-allowed transition-colors"
          >
            {isBrowsingStorageDir.value ? "..." : t("settings.browse")}
          </button>
          <button
            onClick={() => void saveStorageDir()}
            disabled={isSavingStorageDir.value || !storageDirInput.value.trim() || storageDirInput.value.trim() === storageLocation.value}
            class="px-4 py-2 border border-indigo-200 rounded-lg text-sm text-indigo-700 hover:bg-indigo-50 disabled:opacity-60 disabled:cursor-not-allowed transition-colors"
          >
            {isSavingStorageDir.value ? "..." : t("settings.save")}
          </button>
        </div>
        <div class="ml-7 mt-3 space-y-1 text-xs text-gray-500">
          <p>{t("settings.modelsPath", modelDirectory.value || "...")}</p>
          <p>{t("settings.datasetsPath", datasetDirectory.value || "...")}</p>
        </div>
        {storageDirError.value && (
          <p class="mt-3 ml-7 text-sm text-red-600">{storageDirError.value}</p>
        )}
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

      {/* Parallel slots */}
      <div class="mb-10">
        <div class="flex items-center gap-2 mb-1">
          <svg class="w-5 h-5 text-gray-600" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
            <path stroke-linecap="round" stroke-linejoin="round" d="M4 6h16M4 12h16M4 18h16" />
          </svg>
          <span class="font-semibold text-gray-900">{t("settings.parallelSlots")}</span>
        </div>
        <p class="text-sm text-gray-500 mb-4 ml-7">{t("settings.parallelSlotsDesc")}</p>
        <div class="ml-7">
          <input
            type="range"
            min="0"
            max={parallelSteps.length - 1}
            step="1"
            value={parallelIndex.value}
            onInput={(e) => {
              const idx = Number((e.target as HTMLInputElement).value);
              parallelIndex.value = idx;
              saveParallelIndex(idx);
            }}
            class="w-full h-1.5 bg-gray-200 rounded-full appearance-none cursor-pointer accent-indigo-600"
          />
          <div class="flex justify-between mt-2">
            {parallelLabels.map((label) => (
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
          {showTokenInput && (
            <div class="mt-5 border-t border-gray-100 pt-5">
              <label class="mb-2 block text-sm font-medium text-gray-700">{t("chat.cloudTokenLabel")}</label>
              <p class="mb-3 text-sm text-gray-500">{t("settings.tokenInputHint")}</p>
              <div class="flex flex-col gap-3 sm:flex-row sm:items-end">
                <div class="flex-1">
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
                <button
                  onClick={handleSaveCloudToken}
                  disabled={isSavingCloudToken.value}
                  class="px-4 py-2 border border-indigo-200 rounded-lg text-sm text-indigo-700 hover:bg-indigo-50 disabled:opacity-60 disabled:cursor-not-allowed transition-colors"
                >
                  {isSavingCloudToken.value ? t("chat.cloudSavingToken") : t("chat.cloudSaveToken")}
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

      <DirectoryPickerDialog
        open={isStorageDirPickerOpen.value}
        loading={isBrowsingStorageDir.value}
        data={storageDirBrowser.value}
        error={storageDirBrowserError.value}
        onClose={closeStorageDirPicker}
        onBrowse={(path) => void browseStorageDir(path)}
        onSelect={selectStorageDir}
      />
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
