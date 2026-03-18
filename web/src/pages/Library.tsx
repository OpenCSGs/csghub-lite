import { useEffect } from "preact/hooks";
import { signal, computed } from "@preact/signals";
import { getTags, deleteModel, runModel, getPs } from "../api/client";
import type { ModelInfo, RunningModel } from "../api/client";
import { t, locale } from "../i18n";

type FormatFilter = "all" | "gguf" | "safetensors";

const allModels = signal<ModelInfo[]>([]);
const runningModels = signal<RunningModel[]>([]);
const formatFilter = signal<FormatFilter>("all");
const sortField = signal<"name" | "size" | "modified_at">("name");
const sortAsc = signal(true);
const loadingRun = signal<string>("");
const runError = signal<string>("");

const filtered = computed(() => {
  let list = allModels.value;
  if (formatFilter.value !== "all") {
    list = list.filter((m) => m.format === formatFilter.value);
  }
  const field = sortField.value;
  const asc = sortAsc.value;
  return [...list].sort((a, b) => {
    let cmp = 0;
    if (field === "name") cmp = a.name.localeCompare(b.name);
    else if (field === "size") cmp = a.size - b.size;
    else cmp = new Date(a.modified_at).getTime() - new Date(b.modified_at).getTime();
    return asc ? cmp : -cmp;
  });
});

function loadModels() {
  getTags().then((m) => (allModels.value = m)).catch(() => {});
  getPs().then((m) => (runningModels.value = m)).catch(() => {});
}

export function Library() {
  void locale.value;

  useEffect(() => {
    loadModels();
  }, []);

  const handleDelete = async (name: string) => {
    if (!confirm(t("lib.deleteConfirm", name))) return;
    await deleteModel(name);
    allModels.value = allModels.value.filter((m) => m.name !== name);
  };

  const handleRun = async (name: string) => {
    loadingRun.value = name;
    runError.value = "";
    try {
      await runModel(name);
      loadModels();
    } catch (e: any) {
      runError.value = e?.message || t("lib.failedLoad");
    }
    loadingRun.value = "";
  };

  const toggleSort = (field: "name" | "size" | "modified_at") => {
    if (sortField.value === field) {
      sortAsc.value = !sortAsc.value;
    } else {
      sortField.value = field;
      sortAsc.value = true;
    }
  };

  const isRunning = (name: string) => runningModels.value.some((m) => m.name === name);

  return (
    <div class="p-8 max-w-5xl mx-auto">
      <div class="flex items-center justify-between mb-1">
        <div>
          <h1 class="text-2xl font-bold text-gray-900">{t("lib.title")}</h1>
          <p class="text-gray-500 text-sm mt-1">{t("lib.subtitle")}</p>
        </div>
      </div>

      {/* Error Banner */}
      {runError.value && (
        <div class="mt-4 flex items-start gap-2 bg-red-50 border border-red-200 text-red-700 text-sm px-4 py-3 rounded-lg">
          <svg class="w-4 h-4 flex-shrink-0 mt-0.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
            <path stroke-linecap="round" stroke-linejoin="round" d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
          </svg>
          <span class="whitespace-pre-line flex-1">{runError.value}</span>
          <button onClick={() => (runError.value = "")} class="ml-auto text-red-400 hover:text-red-600 flex-shrink-0">&#x2715;</button>
        </div>
      )}

      {/* Filter Tabs */}
      <div class="flex items-center gap-4 mt-6 mb-6">
        <div class="flex bg-gray-100 rounded-lg p-0.5">
          {(["all", "gguf", "safetensors"] as FormatFilter[]).map((f) => (
            <button
              key={f}
              onClick={() => (formatFilter.value = f)}
              class={`px-4 py-1.5 text-sm font-medium rounded-md capitalize transition-colors ${
                formatFilter.value === f
                  ? "bg-white text-gray-900 shadow-sm"
                  : "text-gray-500 hover:text-gray-700"
              }`}
            >
              {f === "all" ? t("lib.all") : f === "gguf" ? "GGUF" : "SafeTensors"}
            </button>
          ))}
        </div>
      </div>

      {/* Table */}
      <div class="bg-white rounded-xl border border-gray-200 overflow-hidden">
        <table class="w-full text-sm">
          <thead>
            <tr class="border-b border-gray-100 text-left text-gray-500 bg-gray-50">
              <SortHeader label={t("lib.modelName")} field="name" current={sortField.value} asc={sortAsc.value} onToggle={toggleSort} />
              <th class="px-4 py-3 font-medium">{t("lib.format")}</th>
              <SortHeader label={t("lib.fileSize")} field="size" current={sortField.value} asc={sortAsc.value} onToggle={toggleSort} />
              <SortHeader label={t("lib.dateTime")} field="modified_at" current={sortField.value} asc={sortAsc.value} onToggle={toggleSort} />
              <th class="px-4 py-3 font-medium text-right">{t("lib.operation")}</th>
            </tr>
          </thead>
          <tbody>
            {filtered.value.length === 0 ? (
              <tr>
                <td colSpan={5} class="text-center py-12 text-gray-400">
                  {t("lib.noModels")}
                </td>
              </tr>
            ) : (
              filtered.value.map((m) => (
                <tr key={m.name} class="border-b border-gray-50 hover:bg-gray-50/50">
                  <td class="px-4 py-3 font-medium text-gray-900">{m.name}</td>
                  <td class="px-4 py-3">
                    <span class={`inline-block px-2 py-0.5 rounded text-xs font-medium ${
                      m.format === "gguf"
                        ? "bg-blue-50 text-blue-700"
                        : "bg-purple-50 text-purple-700"
                    }`}>
                      {m.format?.toUpperCase() || "—"}
                    </span>
                  </td>
                  <td class="px-4 py-3">
                    <span class="bg-indigo-50 text-indigo-700 px-2 py-0.5 rounded text-xs font-medium">
                      {fmtSize(m.size)}
                    </span>
                  </td>
                  <td class="px-4 py-3 text-gray-500">
                    {new Date(m.modified_at).toLocaleDateString("en-US", { day: "numeric", month: "long" })}
                  </td>
                  <td class="px-4 py-3">
                    <div class="flex items-center justify-end gap-3">
                      <button
                        onClick={() => handleDelete(m.name)}
                        class="text-gray-500 hover:text-red-600 text-sm transition-colors"
                      >
                        {t("lib.delete")}
                      </button>
                      {isRunning(m.name) ? (
                        <span class="inline-flex items-center justify-center w-16 px-3 py-1 text-xs rounded bg-green-50 text-green-700 font-medium">
                          {t("lib.running")}
                        </span>
                      ) : (
                        <button
                          onClick={() => handleRun(m.name)}
                          disabled={loadingRun.value === m.name}
                          class="inline-flex items-center justify-center w-16 px-3 py-1 text-xs rounded bg-indigo-600 text-white hover:bg-indigo-700 disabled:opacity-50 transition-colors font-medium"
                        >
                          {loadingRun.value === m.name ? (m.format !== "gguf" ? t("lib.converting") : t("lib.loadingModel")) : t("lib.run")}
                        </button>
                      )}
                    </div>
                  </td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>

      <div class="text-center text-xs text-gray-400 mt-8">
        &copy; OpenCSG &middot; Powered By OpenCSG
      </div>
    </div>
  );
}

function SortHeader({
  label,
  field,
  current,
  asc,
  onToggle,
}: {
  label: string;
  field: string;
  current: string;
  asc: boolean;
  onToggle: (f: "name" | "size" | "modified_at") => void;
}) {
  const active = current === field;
  return (
    <th
      class="px-4 py-3 font-medium cursor-pointer select-none hover:text-gray-700"
      onClick={() => onToggle(field as "name" | "size" | "modified_at")}
    >
      <span class="flex items-center gap-1">
        {label}
        <span class={`text-xs ${active ? "text-indigo-600" : "text-gray-300"}`}>
          {active ? (asc ? "\u25B2" : "\u25BC") : "\u21C5"}
        </span>
      </span>
    </th>
  );
}

function fmtSize(bytes: number): string {
  if (bytes === 0) return "0 B";
  const gb = bytes / (1024 * 1024 * 1024);
  if (gb >= 1) return `${gb.toFixed(1)}GB`;
  const mb = bytes / (1024 * 1024);
  return `${mb.toFixed(0)}MB`;
}
