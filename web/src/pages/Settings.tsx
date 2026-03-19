import { signal } from "@preact/signals";
import { t, locale, setLocale } from "../i18n";
import type { Locale } from "../i18n";

const contextLengthSteps = [4096, 8192, 16384, 32768, 65536, 131072, 262144];
const contextLengthLabels = ["4k", "8k", "16k", "32k", "64k", "128k", "256k"];

const modelLocation = signal("");
const contextIndex = signal(1);

function getDefaultModelLocation(): string {
  return "C:\\Users\\www19\\CSGHub Lite\\models";
}

if (!modelLocation.value) {
  modelLocation.value = getDefaultModelLocation();
}

function resetDefaults() {
  modelLocation.value = getDefaultModelLocation();
  contextIndex.value = 1;
}

export function Settings() {
  void locale.value;

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
        <div class="flex items-center gap-3 ml-7">
          <input
            type="text"
            class="flex-1 px-4 py-2.5 border border-gray-200 rounded-lg text-sm text-gray-700 bg-white focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:border-transparent"
            value={modelLocation.value}
            onInput={(e) => (modelLocation.value = (e.target as HTMLInputElement).value)}
          />
          <button class="flex items-center gap-2 px-4 py-2.5 border border-gray-200 rounded-lg text-sm text-gray-700 hover:bg-gray-50 transition-colors">
            <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
              <path stroke-linecap="round" stroke-linejoin="round" d="M3 7v10a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-6l-2-2H5a2 2 0 00-2 2z" />
            </svg>
            {t("settings.browse")}
          </button>
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
            onInput={(e) => (contextIndex.value = Number((e.target as HTMLInputElement).value))}
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

      {/* Version information */}
      <div class="flex items-center justify-between">
        <div>
          <div class="flex items-center gap-2 mb-1">
            <svg class="w-5 h-5 text-gray-600" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
              <path stroke-linecap="round" stroke-linejoin="round" d="M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z" />
            </svg>
            <span class="font-semibold text-gray-900">{t("settings.versionInfo")}</span>
          </div>
          <p class="text-sm text-gray-500 ml-7">{t("settings.version")}</p>
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
