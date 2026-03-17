import { useEffect } from "preact/hooks";
import { signal, computed } from "@preact/signals";
import {
  getMarketplaceModels,
  getMarketplaceDatasets,
  pullModel,
} from "../api/client";
import type { MarketplaceModel, MarketplaceDataset } from "../api/client";

type Tab = "models" | "datasets";
const activeTab = signal<Tab>("models");
const searchQuery = signal("");
const sortBy = signal("trending");
const page = signal(1);
const perPage = 16;

const models = signal<MarketplaceModel[]>([]);
const datasets = signal<MarketplaceDataset[]>([]);
const total = signal(0);
const loading = signal(false);

const pullingModels = signal<Record<string, { status: string; percent: number }>>({});

const totalPages = computed(() => Math.max(1, Math.ceil(total.value / perPage)));

async function loadData() {
  loading.value = true;
  try {
    if (activeTab.value === "models") {
      const res = await getMarketplaceModels({
        search: searchQuery.value,
        sort: sortBy.value,
        page: page.value,
        per: perPage,
      });
      models.value = res.data || [];
      total.value = res.total;
    } else {
      const res = await getMarketplaceDatasets({
        search: searchQuery.value,
        sort: sortBy.value,
        page: page.value,
        per: perPage,
      });
      datasets.value = res.data || [];
      total.value = res.total;
    }
  } catch {
    /* ignore */
  }
  loading.value = false;
}

export function Marketplace() {
  useEffect(() => {
    loadData();
  }, []);

  useEffect(() => {
    page.value = 1;
    loadData();
  }, [activeTab.value, sortBy.value]);

  const handleSearch = (e: Event) => {
    e.preventDefault();
    page.value = 1;
    loadData();
  };

  const handleDownload = (modelPath: string) => {
    const cur = { ...pullingModels.value };
    cur[modelPath] = { status: "starting", percent: 0 };
    pullingModels.value = cur;

    pullModel(
      modelPath,
      (p) => {
        const pct = p.total && p.total > 0 ? Math.round(((p.completed || 0) / p.total) * 100) : 0;
        const cur = { ...pullingModels.value };
        cur[modelPath] = { status: p.status, percent: pct };
        pullingModels.value = cur;
        if (p.status === "success" || p.status.startsWith("error")) {
          setTimeout(() => {
            const c = { ...pullingModels.value };
            delete c[modelPath];
            pullingModels.value = c;
          }, 3000);
        }
      }
    );
  };

  return (
    <div class="p-8 max-w-5xl mx-auto">
      <h1 class="text-2xl font-bold text-gray-900">Marketplace</h1>
      <p class="text-gray-500 text-sm mt-1 mb-6">
        The model application market allows for quick loading and usage.
      </p>

      {/* Tabs + Search */}
      <div class="flex items-center gap-4 mb-6 flex-wrap">
        <div class="flex bg-gray-100 rounded-lg p-0.5">
          <TabButton label="Models" active={activeTab.value === "models"} onClick={() => (activeTab.value = "models")} />
          <TabButton label="Datasets" active={activeTab.value === "datasets"} onClick={() => (activeTab.value = "datasets")} />
        </div>
        <form onSubmit={handleSearch} class="flex-1 min-w-[200px]">
          <div class="relative">
            <svg class="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
              <path stroke-linecap="round" stroke-linejoin="round" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
            </svg>
            <input
              type="text"
              placeholder="Search..."
              class="w-full pl-10 pr-4 py-2 border border-gray-200 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:border-transparent"
              value={searchQuery.value}
              onInput={(e) => (searchQuery.value = (e.target as HTMLInputElement).value)}
            />
          </div>
        </form>
        <select
          class="border border-gray-200 rounded-lg px-3 py-2 text-sm text-gray-600 focus:outline-none focus:ring-2 focus:ring-indigo-500"
          value={sortBy.value}
          onChange={(e) => (sortBy.value = (e.target as HTMLSelectElement).value)}
        >
          <option value="trending">Trending</option>
          <option value="recently_update">Recently Updated</option>
          <option value="most_download">Most Downloads</option>
          <option value="most_favorite">Most Likes</option>
        </select>
      </div>

      {/* List */}
      {loading.value ? (
        <div class="text-center py-16 text-gray-400">Loading...</div>
      ) : activeTab.value === "models" ? (
        <div class="space-y-0 divide-y divide-gray-100">
          {models.value.map((m) => (
            <ModelCard key={m.id} model={m} pulling={pullingModels.value[m.path]} onDownload={handleDownload} />
          ))}
          {models.value.length === 0 && <p class="text-center py-16 text-gray-400">No models found.</p>}
        </div>
      ) : (
        <div class="space-y-0 divide-y divide-gray-100">
          {datasets.value.map((d) => (
            <DatasetCard key={d.id} dataset={d} />
          ))}
          {datasets.value.length === 0 && <p class="text-center py-16 text-gray-400">No datasets found.</p>}
        </div>
      )}

      {/* Pagination */}
      {totalPages.value > 1 && (
        <div class="flex items-center justify-center gap-2 mt-8">
          <button
            disabled={page.value <= 1}
            onClick={() => { page.value--; loadData(); }}
            class="px-3 py-1.5 text-sm border border-gray-200 rounded-lg disabled:opacity-40 hover:bg-gray-50"
          >
            Previous
          </button>
          <span class="text-sm text-gray-500">
            Page {page.value} of {totalPages.value}
          </span>
          <button
            disabled={page.value >= totalPages.value}
            onClick={() => { page.value++; loadData(); }}
            class="px-3 py-1.5 text-sm border border-gray-200 rounded-lg disabled:opacity-40 hover:bg-gray-50"
          >
            Next
          </button>
        </div>
      )}

      <div class="text-center text-xs text-gray-400 mt-8">
        &copy; OpenCSG &middot; Powered By OpenCSG
      </div>
    </div>
  );
}

function TabButton({ label, active, onClick }: { label: string; active: boolean; onClick: () => void }) {
  return (
    <button
      onClick={onClick}
      class={`px-4 py-1.5 text-sm font-medium rounded-md transition-colors ${
        active ? "bg-white text-gray-900 shadow-sm" : "text-gray-500 hover:text-gray-700"
      }`}
    >
      {label}
    </button>
  );
}

function ModelCard({
  model,
  pulling,
  onDownload,
}: {
  model: MarketplaceModel;
  pulling?: { status: string; percent: number };
  onDownload: (path: string) => void;
}) {
  const tags = model.tags?.filter((t) => t.category === "task" || t.category === "license").slice(0, 3) || [];

  return (
    <div class="flex items-start justify-between py-4">
      <div class="flex-1 min-w-0">
        <div class="flex items-center gap-2">
          <span class="font-medium text-gray-900">{model.path}</span>
        </div>
        {model.description && (
          <p class="text-sm text-gray-500 mt-1 line-clamp-1">{model.description}</p>
        )}
        <div class="flex items-center gap-3 mt-2 text-xs text-gray-400">
          {tags.map((t) => (
            <span key={t.name} class="bg-gray-100 text-gray-600 px-2 py-0.5 rounded">
              {t.show_name || t.name}
            </span>
          ))}
          <span>&middot;</span>
          <span>{new Date(model.updated_at).toLocaleDateString()}</span>
          <span>&middot;</span>
          <span class="flex items-center gap-1">
            <DownloadIcon /> {model.downloads}
          </span>
          <span class="flex items-center gap-1">
            <StarIcon /> {model.likes}
          </span>
        </div>
      </div>
      <div class="ml-4 flex-shrink-0">
        {pulling ? (
          <div class="text-xs text-indigo-600 w-24 text-right">
            {pulling.status === "success" ? (
              <span class="text-green-600">Done!</span>
            ) : pulling.status.startsWith("error") ? (
              <span class="text-red-500">Error</span>
            ) : (
              <span>{pulling.percent}%</span>
            )}
          </div>
        ) : (
          <button
            onClick={() => onDownload(model.path)}
            class="flex items-center gap-1.5 px-4 py-1.5 text-sm border border-gray-200 rounded-lg hover:bg-gray-50 text-gray-700 transition-colors"
          >
            <DownloadIcon /> Download
          </button>
        )}
      </div>
    </div>
  );
}

function DatasetCard({ dataset }: { dataset: MarketplaceDataset }) {
  const tags = dataset.tags?.filter((t) => t.category === "task" || t.category === "license").slice(0, 3) || [];

  return (
    <div class="flex items-start justify-between py-4">
      <div class="flex-1 min-w-0">
        <span class="font-medium text-gray-900">{dataset.path}</span>
        {dataset.description && (
          <p class="text-sm text-gray-500 mt-1 line-clamp-1">{dataset.description}</p>
        )}
        <div class="flex items-center gap-3 mt-2 text-xs text-gray-400">
          {tags.map((t) => (
            <span key={t.name} class="bg-gray-100 text-gray-600 px-2 py-0.5 rounded">
              {t.show_name || t.name}
            </span>
          ))}
          <span>&middot;</span>
          <span>{new Date(dataset.updated_at).toLocaleDateString()}</span>
          <span>&middot;</span>
          <span class="flex items-center gap-1">
            <DownloadIcon /> {dataset.downloads}
          </span>
          <span class="flex items-center gap-1">
            <StarIcon /> {dataset.likes}
          </span>
        </div>
      </div>
    </div>
  );
}

function DownloadIcon() {
  return (
    <svg class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
      <path stroke-linecap="round" stroke-linejoin="round" d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4" />
    </svg>
  );
}

function StarIcon() {
  return (
    <svg class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
      <path stroke-linecap="round" stroke-linejoin="round" d="M11.049 2.927c.3-.921 1.603-.921 1.902 0l1.519 4.674a1 1 0 00.95.69h4.915c.969 0 1.371 1.24.588 1.81l-3.976 2.888a1 1 0 00-.363 1.118l1.518 4.674c.3.922-.755 1.688-1.538 1.118l-3.976-2.888a1 1 0 00-1.176 0l-3.976 2.888c-.783.57-1.838-.197-1.538-1.118l1.518-4.674a1 1 0 00-.363-1.118l-3.976-2.888c-.784-.57-.38-1.81.588-1.81h4.914a1 1 0 00.951-.69l1.519-4.674z" />
    </svg>
  );
}
