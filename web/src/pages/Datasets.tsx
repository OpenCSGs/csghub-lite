import { useEffect } from "preact/hooks";
import { signal } from "@preact/signals";
import { getDatasetTags, searchDatasets, getDatasetFiles, deleteDataset } from "../api/client";
import type { DatasetInfo, DatasetFileEntry } from "../api/client";
import { t, locale } from "../i18n";

type View = { kind: "list" } | { kind: "detail"; dataset: string; path: string };

const allDatasets = signal<DatasetInfo[]>([]);
const searchQuery = signal("");
const sortField = signal<"name" | "size" | "modified_at">("name");
const sortAsc = signal(true);
const currentView = signal<View>({ kind: "list" });
const fileEntries = signal<DatasetFileEntry[]>([]);
const filesLoading = signal(false);
const datasetsLoading = signal(false);

function loadDatasets() {
  datasetsLoading.value = true;
  const query = searchQuery.value.trim();
  const promise = query ? searchDatasets(query, 100, 0) : getDatasetTags();
  promise
    .then((result) => {
      allDatasets.value = Array.isArray(result) ? result : result.datasets;
    })
    .catch(() => {})
    .finally(() => {
      datasetsLoading.value = false;
    });
}

function sortedDatasets(): DatasetInfo[] {
  const field = sortField.value;
  const asc = sortAsc.value;
  return [...allDatasets.value].sort((a, b) => {
    let cmp = 0;
    if (field === "name") cmp = a.name.localeCompare(b.name);
    else if (field === "size") cmp = a.size - b.size;
    else cmp = new Date(a.modified_at).getTime() - new Date(b.modified_at).getTime();
    return asc ? cmp : -cmp;
  });
}

async function loadFiles(dataset: string, path: string) {
  filesLoading.value = true;
  try {
    const resp = await getDatasetFiles(dataset, path);
    const dirs = (resp.entries || []).filter((e) => e.is_dir);
    const files = (resp.entries || []).filter((e) => !e.is_dir);
    dirs.sort((a, b) => a.name.localeCompare(b.name));
    files.sort((a, b) => a.name.localeCompare(b.name));
    fileEntries.value = [...dirs, ...files];
  } catch {
    fileEntries.value = [];
  }
  filesLoading.value = false;
}

export function Datasets() {
  void locale.value;

  useEffect(() => {
    loadDatasets();
    return () => {
      currentView.value = { kind: "list" };
    };
  }, []);

  useEffect(() => {
    const timer = setTimeout(() => {
      loadDatasets();
    }, searchQuery.value.trim() ? 250 : 0);
    return () => clearTimeout(timer);
  }, [searchQuery.value]);

  if (currentView.value.kind === "detail") {
    return <DatasetDetail dataset={currentView.value.dataset} path={currentView.value.path} />;
  }

  return <DatasetList />;
}

function DatasetList() {
  const handleDelete = async (name: string) => {
    if (!confirm(t("ds.deleteConfirm", name))) return;
    await deleteDataset(name);
    allDatasets.value = allDatasets.value.filter((d) => d.name !== name);
  };

  const handleDetails = (name: string) => {
    currentView.value = { kind: "detail", dataset: name, path: "" };
    loadFiles(name, "");
  };

  const toggleSort = (field: "name" | "size" | "modified_at") => {
    if (sortField.value === field) {
      sortAsc.value = !sortAsc.value;
    } else {
      sortField.value = field;
      sortAsc.value = true;
    }
  };

  const datasets = sortedDatasets();

  return (
    <div class="p-8 max-w-5xl mx-auto">
      <div class="mb-1">
        <h1 class="text-2xl font-bold text-gray-900">{t("ds.title")}</h1>
        <p class="text-gray-500 text-sm mt-1">{t("ds.subtitle")}</p>
      </div>

      <div class="flex items-center gap-4 mt-6 mb-6">
        <div class="relative flex-1 min-w-[260px]">
          <svg class="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
            <path stroke-linecap="round" stroke-linejoin="round" d="M21 21l-4.35-4.35m1.85-5.15a7 7 0 11-14 0 7 7 0 0114 0z" />
          </svg>
          <input
            type="text"
            value={searchQuery.value}
            onInput={(e) => (searchQuery.value = (e.currentTarget as HTMLInputElement).value)}
            placeholder={t("ds.search")}
            class="w-full pl-10 pr-24 py-2.5 border border-gray-200 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:border-transparent"
          />
          <span class="absolute right-3 top-1/2 -translate-y-1/2 text-[11px] font-medium text-gray-400 bg-gray-50 px-2 py-0.5 rounded-full">
            {datasetsLoading.value ? t("ds.searching") : t("ds.results", datasets.length)}
          </span>
        </div>
      </div>

      <div class="bg-white rounded-xl border border-gray-200 overflow-hidden">
        <table class="w-full text-sm">
          <thead>
            <tr class="border-b border-gray-100 text-left text-gray-500 bg-gray-50">
              <SortHeader label={t("ds.datasetName")} field="name" current={sortField.value} asc={sortAsc.value} onToggle={toggleSort} />
              <SortHeader label={t("ds.fileSize")} field="size" current={sortField.value} asc={sortAsc.value} onToggle={toggleSort} />
              <SortHeader label={t("ds.dateTime")} field="modified_at" current={sortField.value} asc={sortAsc.value} onToggle={toggleSort} />
              <th class="px-4 py-3 font-medium text-right">{t("ds.operation")}</th>
            </tr>
          </thead>
          <tbody>
            {datasets.length === 0 ? (
              <tr>
                <td colSpan={4} class="text-center py-12 text-gray-400">
                  {t("ds.noDatasets")}
                </td>
              </tr>
            ) : (
              datasets.map((d) => (
                <tr key={d.name} class="border-b border-gray-50 hover:bg-gray-50/50">
                  <td class="px-4 py-3 font-medium text-gray-900">{d.name}</td>
                  <td class="px-4 py-3">
                    <span class="bg-indigo-50 text-indigo-700 px-2 py-0.5 rounded text-xs font-medium">
                      {fmtSize(d.size)}
                    </span>
                  </td>
                  <td class="px-4 py-3 text-gray-500">
                    {new Date(d.modified_at).toLocaleDateString("en-US", { day: "numeric", month: "long" })}
                  </td>
                  <td class="px-4 py-3">
                    <div class="flex items-center justify-end gap-3">
                      <button
                        onClick={() => handleDelete(d.name)}
                        class="text-gray-500 hover:text-red-600 text-sm transition-colors"
                      >
                        {t("ds.delete")}
                      </button>
                      <button
                        onClick={() => handleDetails(d.name)}
                        class="inline-flex items-center justify-center px-3 py-1 text-xs rounded bg-indigo-600 text-white hover:bg-indigo-700 transition-colors font-medium"
                      >
                        {t("ds.details")}
                      </button>
                    </div>
                  </td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>
    </div>
  );
}

function DatasetDetail({ dataset, path }: { dataset: string; path: string }) {
  useEffect(() => {
    loadFiles(dataset, path);
  }, [dataset, path]);

  const navigateTo = (subPath: string) => {
    currentView.value = { kind: "detail", dataset, path: subPath };
  };

  const goBack = () => {
    if (path) {
      const parts = path.split("/").filter(Boolean);
      parts.pop();
      navigateTo(parts.join("/"));
    } else {
      currentView.value = { kind: "list" };
    }
  };

  const breadcrumbs = buildBreadcrumbs(dataset, path);

  return (
    <div class="p-8 max-w-5xl mx-auto">
      <div class="flex items-center gap-3 mb-4">
        <button
          onClick={goBack}
          class="w-8 h-8 flex items-center justify-center rounded-full hover:bg-gray-200 transition-colors text-gray-600"
        >
          <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
            <path stroke-linecap="round" stroke-linejoin="round" d="M15 19l-7-7 7-7" />
          </svg>
        </button>
        <h1 class="text-xl font-bold text-gray-900">{dataset}</h1>
      </div>

      {path && (
        <div class="flex items-center gap-1 mb-4 text-sm text-gray-500">
          {breadcrumbs.map((bc, i) => (
            <span key={i} class="flex items-center gap-1">
              {i > 0 && <span class="text-gray-300 mx-1">/</span>}
              {bc.clickable ? (
                <button
                  onClick={() => navigateTo(bc.path)}
                  class="text-indigo-600 hover:text-indigo-800 hover:underline"
                >
                  {bc.label}
                </button>
              ) : (
                <span class="text-gray-700 font-medium">{bc.label}</span>
              )}
            </span>
          ))}
        </div>
      )}

      <div class="bg-white rounded-xl border border-gray-200 overflow-hidden">
        <table class="w-full text-sm">
          <thead>
            <tr class="border-b border-gray-100 text-left text-gray-500 bg-gray-50">
              <th class="px-4 py-3 font-medium">{t("ds.name")}</th>
              <th class="px-4 py-3 font-medium text-right">{t("ds.size")}</th>
              <th class="px-4 py-3 font-medium text-right">{t("ds.dateModified")}</th>
            </tr>
          </thead>
          <tbody>
            {filesLoading.value ? (
              <tr>
                <td colSpan={3} class="text-center py-12 text-gray-400">
                  {t("ds.loadingFiles")}
                </td>
              </tr>
            ) : fileEntries.value.length === 0 ? (
              <tr>
                <td colSpan={3} class="text-center py-12 text-gray-400">
                  {t("ds.noFiles")}
                </td>
              </tr>
            ) : (
              fileEntries.value.map((f) => (
                <tr key={f.name} class="border-b border-gray-50 hover:bg-gray-50/50">
                  <td class="px-4 py-3">
                    <div class="flex items-center gap-2">
                      {f.is_dir ? <FolderIcon /> : <FileIcon />}
                      {f.is_dir ? (
                        <button
                          onClick={() => navigateTo(path ? `${path}/${f.name}` : f.name)}
                          class="text-indigo-600 hover:text-indigo-800 hover:underline font-medium"
                        >
                          {f.name}
                        </button>
                      ) : (
                        <span class="text-gray-900">{f.name}</span>
                      )}
                    </div>
                  </td>
                  <td class="px-4 py-3 text-right text-gray-500">
                    {f.is_dir ? "—" : fmtSizeDetailed(f.size)}
                  </td>
                  <td class="px-4 py-3 text-right text-gray-500">{fmtRelativeTime(f.modified_at)}</td>
                </tr>
              ))
            )}
          </tbody>
        </table>
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

function FolderIcon() {
  return (
    <svg class="w-4 h-4 text-indigo-500 flex-shrink-0" fill="currentColor" viewBox="0 0 20 20">
      <path d="M2 6a2 2 0 012-2h5l2 2h5a2 2 0 012 2v6a2 2 0 01-2 2H4a2 2 0 01-2-2V6z" />
    </svg>
  );
}

function FileIcon() {
  return (
    <svg class="w-4 h-4 text-gray-400 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
      <path stroke-linecap="round" stroke-linejoin="round" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
    </svg>
  );
}

function buildBreadcrumbs(dataset: string, path: string) {
  const parts = path.split("/").filter(Boolean);
  const name = dataset.split("/").pop() || dataset;
  const crumbs: { label: string; path: string; clickable: boolean }[] = [
    { label: name, path: "", clickable: parts.length > 0 },
  ];
  let accumulated = "";
  for (let i = 0; i < parts.length; i++) {
    accumulated = accumulated ? `${accumulated}/${parts[i]}` : parts[i];
    crumbs.push({
      label: parts[i],
      path: accumulated,
      clickable: i < parts.length - 1,
    });
  }
  return crumbs;
}

function fmtSize(bytes: number): string {
  if (bytes === 0) return "0 B";
  const gb = bytes / (1024 * 1024 * 1024);
  if (gb >= 1) return `${gb.toFixed(1)}GB`;
  const mb = bytes / (1024 * 1024);
  if (mb >= 1) return `${mb.toFixed(1)}MB`;
  const kb = bytes / 1024;
  return `${kb.toFixed(0)}KB`;
}

function fmtSizeDetailed(bytes: number): string {
  if (bytes === 0) return "—";
  const gb = bytes / (1024 * 1024 * 1024);
  if (gb >= 1) return `${gb.toFixed(0)} GB`;
  const mb = bytes / (1024 * 1024);
  if (mb >= 1) return `${mb.toFixed(mb >= 100 ? 0 : 1)} MB`;
  const kb = bytes / 1024;
  if (kb >= 1) return `${kb.toFixed(kb >= 100 ? 0 : 1)} kB`;
  return `${bytes} Bytes`;
}

function fmtRelativeTime(dateStr: string): string {
  const date = new Date(dateStr);
  const now = new Date();
  const diffMs = now.getTime() - date.getTime();
  const diffMin = Math.floor(diffMs / 60000);
  if (diffMin < 1) return t("ds.lessThanMinute");
  if (diffMin < 60) return t("ds.minutesAgo", diffMin);
  const diffHours = Math.floor(diffMin / 60);
  if (diffHours < 24) return t("ds.hoursAgo", diffHours);
  const diffDays = Math.floor(diffHours / 24);
  return t("ds.daysAgo", diffDays);
}
