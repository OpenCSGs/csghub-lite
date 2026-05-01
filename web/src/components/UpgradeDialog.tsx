import { t } from "../i18n";

export interface UpgradeProgress {
  status: "idle" | "checking" | "confirming" | "upgrading" | "success" | "error";
  currentVersion: string;
  latestVersion?: string;
  hasUpdate: boolean;
  percent: number;
  message: string;
  error?: string;
}

type UpgradeDialogProps = {
  open: boolean;
  progress: UpgradeProgress;
  onConfirm: () => void;
  onClose: () => void;
};

export function UpgradeDialog({ open, progress, onConfirm, onClose }: UpgradeDialogProps) {
  if (!open) return null;

  const isUpgrading = progress.status === "upgrading";
  const isSuccess = progress.status === "success";
  const isError = progress.status === "error";
  const needsConfirm = progress.status === "confirming";

  return (
    <div class="fixed inset-0 z-50 flex items-center justify-center bg-gray-900/40 p-4" onClick={onClose}>
      <div
        class="w-full max-w-md overflow-hidden rounded-2xl border border-gray-200 bg-white shadow-2xl"
        onClick={(e) => e.stopPropagation()}
      >
        <div class="flex items-start justify-between gap-4 border-b border-gray-100 px-5 py-4">
          <div>
            <h2 class="text-lg font-semibold text-gray-900">{t("upgrade.title")}</h2>
            <p class="mt-1 text-sm text-gray-500">{t("upgrade.subtitle")}</p>
          </div>
          <button
            type="button"
            onClick={onClose}
            disabled={isUpgrading}
            class="rounded-lg p-2 text-gray-500 transition-colors hover:bg-gray-100 hover:text-gray-700 disabled:opacity-50 disabled:cursor-not-allowed"
            aria-label={t("dash.close")}
            title={t("dash.close")}
          >
            <svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
              <path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </div>

        <div class="px-5 py-4 space-y-4">
          {/* Version info */}
          <div class="flex items-center justify-between text-sm">
            <span class="text-gray-500">{t("upgrade.currentVersion")}</span>
            <span class="font-mono text-gray-900">{progress.currentVersion}</span>
          </div>

          {progress.latestVersion && (
            <div class="flex items-center justify-between text-sm">
              <span class="text-gray-500">{t("upgrade.latestVersion")}</span>
              <span class="font-mono text-indigo-600">{progress.latestVersion}</span>
            </div>
          )}

          {/* Error message */}
          {isError && progress.error && (
            <div class="rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">
              {progress.error}
            </div>
          )}

          {/* Progress bar */}
          {isUpgrading && (
            <div class="space-y-2">
              <div class="flex items-center justify-between">
                <span class="text-sm font-medium text-indigo-600">{progress.message || t("upgrade.upgrading")}</span>
                <span class="text-sm text-gray-500">{progress.percent}%</span>
              </div>
              <div class="w-full h-2 bg-gray-200 rounded-full overflow-hidden">
                <div
                  class="h-full bg-indigo-500 rounded-full transition-all duration-300"
                  style={{ width: `${Math.max(progress.percent, 3)}%` }}
                />
              </div>
            </div>
          )}

          {/* Success message */}
          {isSuccess && (
            <div class="rounded-lg border border-green-200 bg-green-50 px-4 py-3 text-sm text-green-700">
              {t("upgrade.successMessage")}
            </div>
          )}
        </div>

        <div class="flex justify-end gap-3 border-t border-gray-100 px-5 py-4">
          {needsConfirm && (
            <>
              <button
                type="button"
                onClick={onClose}
                class="rounded-lg border border-gray-200 px-4 py-2 text-sm text-gray-700 transition-colors hover:bg-gray-50"
              >
                {t("upgrade.cancel")}
              </button>
              <button
                type="button"
                onClick={onConfirm}
                class="rounded-lg bg-indigo-600 px-4 py-2 text-sm text-white transition-colors hover:bg-indigo-700"
              >
                {t("upgrade.confirm")}
              </button>
            </>
          )}

          {isUpgrading && (
            <button
              type="button"
              disabled
              class="rounded-lg bg-gray-100 px-4 py-2 text-sm text-gray-500 cursor-not-allowed"
            >
              {t("upgrade.upgrading")}
            </button>
          )}

          {isSuccess && (
            <button
              type="button"
              onClick={() => window.location.reload()}
              class="rounded-lg bg-indigo-600 px-4 py-2 text-sm text-white transition-colors hover:bg-indigo-700"
            >
              {t("upgrade.refresh")}
            </button>
          )}

          {isError && (
            <button
              type="button"
              onClick={onClose}
              class="rounded-lg border border-gray-200 px-4 py-2 text-sm text-gray-700 transition-colors hover:bg-gray-50"
            >
              {t("dash.close")}
            </button>
          )}
        </div>
      </div>
    </div>
  );
}
