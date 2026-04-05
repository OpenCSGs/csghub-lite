import type { LocalDirectoryBrowseResponse } from "../api/client";
import { t } from "../i18n";

type DirectoryPickerDialogProps = {
  open: boolean;
  loading: boolean;
  data: LocalDirectoryBrowseResponse | null;
  error: string;
  onClose: () => void;
  onBrowse: (path: string) => void;
  onSelect: (path: string) => void;
};

export function DirectoryPickerDialog({
  open,
  loading,
  data,
  error,
  onClose,
  onBrowse,
  onSelect,
}: DirectoryPickerDialogProps) {
  if (!open) return null;

  const currentPath = data?.current_path || "";
  const parentPath = data?.parent_path || "";
  const homePath = data?.home_path || "";
  const shortcutPaths = Array.from(new Set([homePath, ...(data?.roots || [])].filter(Boolean)));

  return (
    <div class="fixed inset-0 z-50 flex items-center justify-center bg-gray-900/40 p-4" onClick={onClose}>
      <div
        class="w-full max-w-3xl overflow-hidden rounded-2xl border border-gray-200 bg-white shadow-2xl"
        onClick={(e) => e.stopPropagation()}
      >
        <div class="flex items-start justify-between gap-4 border-b border-gray-100 px-5 py-4">
          <div>
            <h2 class="text-lg font-semibold text-gray-900">{t("settings.directoryBrowserTitle")}</h2>
            <p class="mt-1 text-sm text-gray-500">{t("settings.directoryBrowserDesc")}</p>
          </div>
          <button
            type="button"
            onClick={onClose}
            class="rounded-lg p-2 text-gray-500 transition-colors hover:bg-gray-100 hover:text-gray-700"
            aria-label={t("dash.close")}
            title={t("dash.close")}
          >
            <svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
              <path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </div>

        <div class="space-y-4 px-5 py-4">
          {error && (
            <div class="rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">
              {error}
            </div>
          )}

          {shortcutPaths.length > 0 && (
            <div class="flex flex-wrap gap-2">
              {shortcutPaths.map((path) => (
                <button
                  key={path}
                  type="button"
                  onClick={() => onBrowse(path)}
                  class="rounded-full border border-gray-200 px-3 py-1.5 text-sm text-gray-700 transition-colors hover:bg-gray-50"
                >
                  {path === homePath ? t("settings.home") : path}
                </button>
              ))}
            </div>
          )}

          <div>
            <p class="mb-2 text-sm font-medium text-gray-700">{t("settings.currentFolder")}</p>
            <div class="flex items-center gap-3">
              <button
                type="button"
                onClick={() => parentPath && onBrowse(parentPath)}
                disabled={loading || !parentPath}
                class="rounded-lg border border-gray-200 px-3 py-2 text-sm text-gray-700 transition-colors hover:bg-gray-50 disabled:cursor-not-allowed disabled:opacity-60"
                title={t("settings.upOneLevel")}
              >
                ..
              </button>
              <div class="min-h-[42px] flex-1 rounded-lg border border-gray-200 bg-gray-50 px-4 py-2 text-sm text-gray-700">
                <span class="font-mono break-all">{currentPath || "..."}</span>
              </div>
            </div>
          </div>

          <div class="overflow-hidden rounded-xl border border-gray-200">
            <div class="max-h-[320px] overflow-y-auto">
              {loading ? (
                <div class="px-4 py-10 text-center text-sm text-gray-500">{t("settings.loadingDirectories")}</div>
              ) : (data?.entries || []).length === 0 ? (
                <div class="px-4 py-10 text-center text-sm text-gray-500">{t("settings.noDirectories")}</div>
              ) : (
                (data?.entries || []).map((entry) => (
                  <button
                    key={entry.path}
                    type="button"
                    onClick={() => onBrowse(entry.path)}
                    class="flex w-full items-center gap-3 border-b border-gray-100 px-4 py-3 text-left transition-colors hover:bg-gray-50 last:border-b-0"
                  >
                    <FolderIcon />
                    <div class="min-w-0">
                      <div class="font-medium text-gray-900 break-all">{entry.name}</div>
                      <div class="mt-0.5 text-xs text-gray-500 break-all">{entry.path}</div>
                    </div>
                  </button>
                ))
              )}
            </div>
          </div>
        </div>

        <div class="flex justify-end gap-3 border-t border-gray-100 px-5 py-4">
          <button
            type="button"
            onClick={onClose}
            class="rounded-lg border border-gray-200 px-4 py-2 text-sm text-gray-700 transition-colors hover:bg-gray-50"
          >
            {t("settings.cancel")}
          </button>
          <button
            type="button"
            onClick={() => currentPath && onSelect(currentPath)}
            disabled={!currentPath}
            class="rounded-lg border border-indigo-200 px-4 py-2 text-sm text-indigo-700 transition-colors hover:bg-indigo-50 disabled:cursor-not-allowed disabled:opacity-60"
          >
            {t("settings.useCurrentFolder")}
          </button>
        </div>
      </div>
    </div>
  );
}

function FolderIcon() {
  return (
    <svg class="h-5 w-5 flex-shrink-0 text-amber-500" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
      <path stroke-linecap="round" stroke-linejoin="round" d="M3 7v10a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-6l-2-2H5a2 2 0 00-2 2z" />
    </svg>
  );
}
