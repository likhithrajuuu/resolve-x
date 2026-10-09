"use client";

import { useState } from "react";

export function snippets(key: string, ingest: string, api: string) {
  return {
    node: {
      label: "Node.js",
      install: "npm install @resolve-x/sdk",
      code: `// first line of your entrypoint (before other imports)
const resolvex = require("@resolve-x/sdk");

resolvex.init({
  apiKey: "${key}",
  serviceName: "checkout",
  version: process.env.GIT_SHA,        // enables deployment tracking
  endpoint: "${ingest}",
  apiUrl: "${api}",
  markDeployment: true,
});

// Zero code change alternative:
//   RESOLVEX_API_KEY=${key} RESOLVEX_SERVICE=checkout node -r @resolve-x/sdk/register app.js`,
    },
    python: {
      label: "Python",
      install: 'pip install "resolvex[web,db]"',
      code: `import resolvex

resolvex.init(
    api_key="${key}",
    service_name="checkout",
    version=os.environ.get("GIT_SHA"),   # enables deployment tracking
    endpoint="${ingest}",
    api_url="${api}",
    mark_deployment_on_start=True,
)
# Import Flask / FastAPI / requests / SQLAlchemy *after* init().`,
    },
    otel: {
      label: "Any OpenTelemetry SDK",
      install: "# Java, Go, .NET, Ruby, PHP, Rust … use their standard OTel SDK",
      code: `export OTEL_EXPORTER_OTLP_ENDPOINT=${ingest}
export OTEL_EXPORTER_OTLP_HEADERS="x-resolvex-key=${key}"
export OTEL_SERVICE_NAME=checkout
export OTEL_RESOURCE_ATTRIBUTES="service.version=1.4.2,deployment.environment=production"`,
    },
  } as const;
}

export function SdkSnippets({ apiKey, ingest, api }: { apiKey: string; ingest: string; api: string }) {
  const s = snippets(apiKey, ingest, api);
  const [tab, setTab] = useState<keyof typeof s>("node");
  const [copied, setCopied] = useState(false);
  const cur = s[tab];
  return (
    <div>
      <div role="tablist" aria-label="Language" className="flex gap-1 border-b border-line text-sm">
        {(Object.keys(s) as (keyof typeof s)[]).map((k) => (
          <button key={k} role="tab" aria-selected={tab === k} onClick={() => setTab(k)} className={`-mb-px border-b-2 px-4 py-2 ${tab === k ? "border-accent text-fg" : "border-transparent text-muted hover:text-fg"}`}>{s[k].label}</button>
        ))}
      </div>
      <p className="mt-3 font-mono text-xs text-muted">$ {cur.install}</p>
      <pre className="mt-2 overflow-x-auto rounded-md bg-bg p-4 font-mono text-xs leading-relaxed">{cur.code}</pre>
      <button className="mt-2 text-xs text-muted hover:text-fg" onClick={async () => { await navigator.clipboard?.writeText(cur.code); setCopied(true); setTimeout(() => setCopied(false), 1500); }}>{copied ? "Copied" : "Copy"}</button>
    </div>
  );
}
