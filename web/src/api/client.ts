export interface ModelInfo {
  name: string;
  model: string;
  size: number;
  format: string;
  modified_at: string;
  pipeline_tag?: string;
  has_mmproj?: boolean;
}

export interface RunningModel {
  name: string;
  model: string;
  size: number;
  format: string;
  expires_at: string;
}

export interface MarketplaceModel {
  id: number;
  name: string;
  path: string;
  description: string;
  likes: number;
  downloads: number;
  tags: { name: string; category: string; show_name: string }[];
  license: string;
  created_at: string;
  updated_at: string;
}

export interface MarketplaceDataset {
  id: number;
  name: string;
  path: string;
  description: string;
  likes: number;
  downloads: number;
  tags: { name: string; category: string; show_name: string }[];
  license: string;
  created_at: string;
  updated_at: string;
}

export interface SystemInfo {
  cpu_cores: number;
  cpu_usage: number;
  cpu_clock: string;
  ram_used: number;
  ram_total: number;
  ram_info: string;
  gpu_name: string;
  gpu_vram_used: number;
  gpu_vram_total: number;
}

export type ChatContent = string | ContentPart[];

export interface ContentPart {
  type: "text" | "image_url";
  text?: string;
  image_url?: { url: string };
}

export interface ChatMessage {
  role: string;
  content: ChatContent;
}

export interface PullProgress {
  status: string;
  digest?: string;
  total?: number;
  completed?: number;
}

async function fetchJSON<T>(url: string, init?: RequestInit): Promise<T> {
  const resp = await fetch(url, init);
  if (!resp.ok) {
    const err = await resp.json().catch(() => ({ error: resp.statusText }));
    throw new Error(err.error || resp.statusText);
  }
  return resp.json();
}

export async function getTags(): Promise<ModelInfo[]> {
  const data = await fetchJSON<{ models: ModelInfo[] }>("/api/tags");
  return data.models || [];
}

export async function getPs(): Promise<RunningModel[]> {
  const data = await fetchJSON<{ models: RunningModel[] }>("/api/ps");
  return data.models || [];
}

export async function stopModel(model: string): Promise<void> {
  await fetchJSON("/api/stop", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ model }),
  });
}

export async function deleteModel(model: string): Promise<void> {
  await fetchJSON("/api/delete", {
    method: "DELETE",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ model }),
  });
}

export async function showModel(model: string) {
  return fetchJSON<{ details: ModelInfo }>("/api/show", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ model }),
  });
}

export function pullModel(
  model: string,
  onProgress: (p: PullProgress) => void,
  signal?: AbortSignal
): Promise<void> {
  return new Promise((resolve, reject) => {
    fetch("/api/pull", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ model }),
      signal,
    })
      .then((resp) => {
        if (!resp.ok || !resp.body) {
          reject(new Error("pull failed"));
          return;
        }
        const reader = resp.body.getReader();
        const decoder = new TextDecoder();
        let buf = "";
        let lastUpdate = 0;
        let pending: PullProgress | null = null;
        let flushTimer = 0;

        function flushPending() {
          if (pending) {
            onProgress(pending);
            pending = null;
          }
        }

        function processLine(line: string) {
          if (!line.startsWith("data: ")) return;
          try {
            const p: PullProgress = JSON.parse(line.slice(6));
            if (p.status === "success" || p.status.startsWith("error")) {
              clearTimeout(flushTimer);
              onProgress(p);
              return;
            }
            const now = Date.now();
            if (now - lastUpdate >= 200) {
              lastUpdate = now;
              onProgress(p);
            } else {
              pending = p;
              clearTimeout(flushTimer);
              flushTimer = window.setTimeout(flushPending, 200);
            }
          } catch {
            /* skip */
          }
        }

        function read(): Promise<void> {
          return reader.read().then(({ done, value }) => {
            if (done) {
              clearTimeout(flushTimer);
              flushPending();
              resolve();
              return;
            }
            buf += decoder.decode(value, { stream: true });
            const lines = buf.split("\n");
            buf = lines.pop() || "";
            for (const line of lines) {
              processLine(line);
            }
            return read();
          });
        }

        read().catch((err) => {
          clearTimeout(flushTimer);
          reject(err);
        });
      })
      .catch(reject);
  });
}

function stripImagesFromOldMessages(msgs: ChatMessage[]): ChatMessage[] {
  if (msgs.length <= 1) return msgs;
  return msgs.map((m, i) => {
    if (i === msgs.length - 1) return m;
    if (!Array.isArray(m.content)) return m;
    const textParts = (m.content as ContentPart[])
      .filter((p) => p.type === "text")
      .map((p) => p.text || "")
      .join("");
    return { ...m, content: textParts || "(image)" };
  });
}

export function streamChat(
  model: string,
  messages: ChatMessage[],
  options: { temperature?: number; top_p?: number; max_tokens?: number; system?: string },
  onToken: (token: string, done: boolean) => void,
  signal?: AbortSignal
): Promise<void> {
  let msgs = stripImagesFromOldMessages([...messages]);
  if (options.system) {
    msgs.unshift({ role: "system", content: options.system });
  }

  return new Promise((resolve, reject) => {
    fetch("/api/chat", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        model,
        messages: msgs,
        stream: true,
        options: {
          temperature: options.temperature,
          top_p: options.top_p,
          max_tokens: options.max_tokens,
        },
      }),
      signal,
    })
      .then(async (resp) => {
        if (!resp.ok) {
          const errText = await resp.text().catch(() => resp.statusText);
          reject(new Error(`Error ${resp.status}: ${errText}`));
          return;
        }
        if (!resp.body) {
          reject(new Error("No response body"));
          return;
        }
        const reader = resp.body.getReader();
        const decoder = new TextDecoder();
        let buf = "";

        function read(): Promise<void> {
          return reader.read().then(({ done, value }) => {
            if (done) {
              resolve();
              return;
            }
            buf += decoder.decode(value, { stream: true });
            const lines = buf.split("\n");
            buf = lines.pop() || "";
            for (const line of lines) {
              if (line.startsWith("data: ")) {
                try {
                  const data = JSON.parse(line.slice(6));
                  if (data.message?.content) {
                    onToken(data.message.content, false);
                  }
                  if (data.done) {
                    onToken("", true);
                  }
                } catch {
                  /* skip */
                }
              }
            }
            return read();
          });
        }

        read().catch(reject);
      })
      .catch(reject);
  });
}

export interface LoadProgress {
  status: string;
  step?: string;
  current?: number;
  total?: number;
}

export function loadModel(
  model: string,
  onProgress: (p: LoadProgress) => void,
  signal?: AbortSignal
): Promise<void> {
  return new Promise((resolve, reject) => {
    fetch("/api/load", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ model, stream: true }),
      signal,
    })
      .then((resp) => {
        if (!resp.ok || !resp.body) {
          reject(new Error("load failed"));
          return;
        }
        const reader = resp.body.getReader();
        const decoder = new TextDecoder();
        let buf = "";

        function processLine(line: string) {
          if (!line.startsWith("data: ")) return;
          try {
            const p: LoadProgress = JSON.parse(line.slice(6));
            onProgress(p);
            if (p.status === "ready") {
              resolve();
            } else if (p.status.startsWith("error")) {
              reject(new Error(p.status));
            }
          } catch {
            /* skip */
          }
        }

        function read(): Promise<void> {
          return reader.read().then(({ done, value }) => {
            if (done) {
              resolve();
              return;
            }
            buf += decoder.decode(value, { stream: true });
            const lines = buf.split("\n");
            buf = lines.pop() || "";
            for (const line of lines) {
              processLine(line);
            }
            return read();
          });
        }

        read().catch(reject);
      })
      .catch(reject);
  });
}

export async function runModel(model: string): Promise<void> {
  const stream = false;
  await fetchJSON("/api/generate", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ model, prompt: "hi", stream }),
  });
}

export interface DatasetInfo {
  name: string;
  dataset: string;
  size: number;
  files: number;
  modified_at: string;
}

export async function getDatasetTags(): Promise<DatasetInfo[]> {
  const data = await fetchJSON<{ datasets: DatasetInfo[] }>("/api/datasets");
  return data.datasets || [];
}

export function pullDataset(
  dataset: string,
  onProgress: (p: PullProgress) => void,
  signal?: AbortSignal
): Promise<void> {
  return new Promise((resolve, reject) => {
    fetch("/api/datasets/pull", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ dataset }),
      signal,
    })
      .then((resp) => {
        if (!resp.ok || !resp.body) {
          reject(new Error("pull failed"));
          return;
        }
        const reader = resp.body.getReader();
        const decoder = new TextDecoder();
        let buf = "";
        let lastUpdate = 0;
        let pending: PullProgress | null = null;
        let flushTimer = 0;

        function flushPending() {
          if (pending) {
            onProgress(pending);
            pending = null;
          }
        }

        function processLine(line: string) {
          if (!line.startsWith("data: ")) return;
          try {
            const p: PullProgress = JSON.parse(line.slice(6));
            if (p.status === "success" || p.status.startsWith("error")) {
              clearTimeout(flushTimer);
              onProgress(p);
              return;
            }
            const now = Date.now();
            if (now - lastUpdate >= 200) {
              lastUpdate = now;
              onProgress(p);
            } else {
              pending = p;
              clearTimeout(flushTimer);
              flushTimer = window.setTimeout(flushPending, 200);
            }
          } catch {
            /* skip */
          }
        }

        function read(): Promise<void> {
          return reader.read().then(({ done, value }) => {
            if (done) {
              clearTimeout(flushTimer);
              flushPending();
              resolve();
              return;
            }
            buf += decoder.decode(value, { stream: true });
            const lines = buf.split("\n");
            buf = lines.pop() || "";
            for (const line of lines) {
              processLine(line);
            }
            return read();
          });
        }

        read().catch((err) => {
          clearTimeout(flushTimer);
          reject(err);
        });
      })
      .catch(reject);
  });
}

export async function deleteDataset(dataset: string): Promise<void> {
  await fetchJSON("/api/datasets/delete", {
    method: "DELETE",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ dataset }),
  });
}

export interface DatasetFileEntry {
  name: string;
  size: number;
  is_dir: boolean;
  modified_at: string;
}

export interface DatasetFilesResponse {
  dataset: string;
  path: string;
  entries: DatasetFileEntry[];
}

export async function getDatasetFiles(
  dataset: string,
  path: string
): Promise<DatasetFilesResponse> {
  return fetchJSON<DatasetFilesResponse>("/api/datasets/files", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ dataset, path }),
  });
}

export async function getMarketplaceModels(params: {
  search?: string;
  sort?: string;
  page?: number;
  per?: number;
}): Promise<{ data: MarketplaceModel[]; total: number }> {
  const q = new URLSearchParams();
  if (params.search) q.set("search", params.search);
  q.set("sort", params.sort || "trending");
  q.set("page", String(params.page || 1));
  q.set("per", String(params.per || 16));
  const resp = await fetchJSON<{ data: MarketplaceModel[]; total: number }>(
    `/api/marketplace/models?${q}`
  );
  return resp;
}

export async function getMarketplaceDatasets(params: {
  search?: string;
  sort?: string;
  page?: number;
  per?: number;
}): Promise<{ data: MarketplaceDataset[]; total: number }> {
  const q = new URLSearchParams();
  if (params.search) q.set("search", params.search);
  q.set("sort", params.sort || "trending");
  q.set("page", String(params.page || 1));
  q.set("per", String(params.per || 16));
  const resp = await fetchJSON<{ data: MarketplaceDataset[]; total: number }>(
    `/api/marketplace/datasets?${q}`
  );
  return resp;
}

export async function getSystemInfo(): Promise<SystemInfo> {
  return fetchJSON<SystemInfo>("/api/system");
}

export function streamLogs(
  onLog: (line: string) => void,
  signal?: AbortSignal
): void {
  const evtSource = new EventSource("/api/logs");
  evtSource.onmessage = (e) => onLog(e.data);
  evtSource.onerror = () => {
    evtSource.close();
  };
  signal?.addEventListener("abort", () => evtSource.close());
}
