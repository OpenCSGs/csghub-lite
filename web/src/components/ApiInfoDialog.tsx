import { signal } from "@preact/signals";
import { t } from "../i18n";

export function ApiInfoDialog({
  model,
  isVision,
  isEmbedding,
  isASR,
  onClose,
}: {
  model: string;
  isVision?: boolean;
  isEmbedding?: boolean;
  isASR?: boolean;
  onClose: () => void;
}) {
  const baseUrl = `${location.protocol}//${location.host}`;

  const textMsg = `{"role": "user", "content": "Hello!"}`;
  const visionMsg = `{"role": "user", "content": [
        {"type": "text", "text": "What is in this image?"},
        {"type": "image_url", "image_url": {"url": "data:image/png;base64,<BASE64_DATA>"}}
      ]}`;

  const curlExample = isASR
    ? `curl ${baseUrl}/v1/audio/transcriptions \\
  -F model="${model}" \\
  -F file="@audio.mp3" \\
  -F response_format="json"`
    : isEmbedding
    ? `curl ${baseUrl}/v1/embeddings \\
  -H "Content-Type: application/json" \\
  -d '{
    "model": "${model}",
    "input": ["Hello!"],
    "encoding_format": "float"
  }'`
    : `curl ${baseUrl}/v1/chat/completions \\
  -H "Content-Type: application/json" \\
  -d '{
    "model": "${model}",
    "messages": [
      ${isVision ? visionMsg : textMsg}
    ],
    "stream": true
  }'`;

  const pythonTextMsg = `{"role": "user", "content": "Hello!"}`;
  const pythonVisionMsg = `{
            "role": "user",
            "content": [
                {"type": "text", "text": "What is in this image?"},
                {
                    "type": "image_url",
                    "image_url": {"url": f"data:image/png;base64,{img_b64}"}
                }
            ]
        }`;

  const pythonExample = isASR
    ? `from openai import OpenAI

client = OpenAI(
    base_url="${baseUrl}/v1",
    api_key="unused"
)

with open("audio.mp3", "rb") as audio:
    response = client.audio.transcriptions.create(
        model="${model}",
        file=audio,
        response_format="json"
    )

print(response.text)`
    : isEmbedding
    ? `from openai import OpenAI

client = OpenAI(
    base_url="${baseUrl}/v1",
    api_key="unused"
)

response = client.embeddings.create(
    model="${model}",
    input=["Hello!"],
    encoding_format="float"
)

print(response.data[0].embedding)`
    : isVision
    ? `import base64
from openai import OpenAI

client = OpenAI(
    base_url="${baseUrl}/v1",
    api_key="unused"
)

with open("image.png", "rb") as f:
    img_b64 = base64.b64encode(f.read()).decode()

response = client.chat.completions.create(
    model="${model}",
    messages=[
        ${pythonVisionMsg}
    ],
    stream=True
)

for chunk in response:
    if chunk.choices[0].delta.content:
        print(chunk.choices[0].delta.content, end="")`
    : `from openai import OpenAI

client = OpenAI(
    base_url="${baseUrl}/v1",
    api_key="unused"
)

response = client.chat.completions.create(
    model="${model}",
    messages=[
        ${pythonTextMsg}
    ],
    stream=True
)

for chunk in response:
    if chunk.choices[0].delta.content:
        print(chunk.choices[0].delta.content, end="")`;

  const jsTextMsg = `{ role: "user", content: "Hello!" }`;
  const jsVisionMsg = `{
      role: "user",
      content: [
        { type: "text", text: "What is in this image?" },
        { type: "image_url", image_url: { url: \`data:image/png;base64,\${imgBase64}\` } }
      ]
    }`;

  const jsExample = isASR
    ? `const form = new FormData();
form.set("model", "${model}");
form.set("file", audioFile); // File from <input type="file">
form.set("response_format", "json");

const response = await fetch("${baseUrl}/v1/audio/transcriptions", {
  method: "POST",
  body: form
});

const data = await response.json();
console.log(data.text);`
    : isEmbedding
    ? `const response = await fetch("${baseUrl}/v1/embeddings", {
  method: "POST",
  headers: { "Content-Type": "application/json" },
  body: JSON.stringify({
    model: "${model}",
    input: ["Hello!"],
    encoding_format: "float"
  })
});

const data = await response.json();
console.log(data.data[0].embedding);`
    : isVision
    ? `const imgBase64 = "..."; // Base64-encoded image data

const response = await fetch("${baseUrl}/v1/chat/completions", {
  method: "POST",
  headers: { "Content-Type": "application/json" },
  body: JSON.stringify({
    model: "${model}",
    messages: [
      ${jsVisionMsg}
    ],
    stream: false
  })
});

const data = await response.json();
console.log(data.choices[0].message.content);`
    : `const response = await fetch("${baseUrl}/v1/chat/completions", {
  method: "POST",
  headers: { "Content-Type": "application/json" },
  body: JSON.stringify({
    model: "${model}",
    messages: [
      ${jsTextMsg}
    ],
    stream: false
  })
});

const data = await response.json();
console.log(data.choices[0].message.content);`;

  return (
    <div
      class="fixed inset-0 z-50 flex items-center justify-center bg-black/40"
      onClick={(e) => { if (e.target === e.currentTarget) onClose(); }}
    >
      <div class="bg-white rounded-2xl shadow-2xl max-w-2xl w-full mx-4 max-h-[85vh] flex flex-col">
        <div class="flex items-center justify-between px-6 py-4 border-b border-gray-100">
          <div>
            <h3 class="text-lg font-bold text-gray-900">{t("dash.apiTitle")}</h3>
            <p class="text-sm text-gray-500 mt-0.5">
              {t("dash.apiModel")}: <span class="font-mono text-indigo-600">{model}</span>
              {isVision && <span class="ml-2 px-1.5 py-0.5 text-xs bg-purple-50 text-purple-700 rounded">Vision</span>}
              {isEmbedding && <span class="ml-2 px-1.5 py-0.5 text-xs bg-emerald-50 text-emerald-700 rounded">Embedding</span>}
              {isASR && <span class="ml-2 px-1.5 py-0.5 text-xs bg-cyan-50 text-cyan-700 rounded">ASR</span>}
            </p>
          </div>
        </div>
        <div class="flex-1 overflow-auto px-6 py-4 space-y-5">
          <CodeBlock title={t("dash.apiCurl")} code={curlExample} />
          <CodeBlock title={t("dash.apiPython")} code={pythonExample} />
          <CodeBlock title={t("dash.apiJs")} code={jsExample} />
        </div>
      </div>
    </div>
  );
}

function CodeBlock({ title, code }: { title: string; code: string }) {
  const copied = signal(false);
  const handleCopy = () => {
    navigator.clipboard.writeText(code).then(() => {
      copied.value = true;
      setTimeout(() => { copied.value = false; }, 2000);
    }).catch(() => {});
  };

  return (
    <div>
      <div class="flex items-center justify-between mb-1.5">
        <span class="text-sm font-medium text-gray-700">{title}</span>
        <button onClick={handleCopy} class={`text-xs transition-colors flex items-center gap-1 ${copied.value ? "text-green-600" : "text-gray-400 hover:text-indigo-600"}`}>
          {copied.value ? (
            <svg class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
              <path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7" />
            </svg>
          ) : (
            <svg class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
              <path stroke-linecap="round" stroke-linejoin="round" d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z" />
            </svg>
          )}
          {copied.value ? t("dash.copied") : t("dash.copy")}
        </button>
      </div>
      <pre class="bg-gray-900 text-gray-100 rounded-lg p-4 text-xs leading-5 overflow-x-auto font-mono whitespace-pre-wrap break-all">
        {code}
      </pre>
    </div>
  );
}
