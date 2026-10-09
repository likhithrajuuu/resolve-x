"use client";

import { useState, type FormEvent } from "react";
import { api, useApi } from "@/lib/api";
import { timeAgo } from "@/lib/format";
import { SdkSnippets } from "@/components/SdkSnippets";
import { btn, btnGhost, Card, ErrorNote, input, Loading, PageHeader } from "@/components/ui";

type Project = { id: string; name: string };
type Env = { id: string; name: string };
type Key = { id: string; environmentId: string; prefix: string; status: string; createdAt: string };

export default function SettingsPage() {
  const projects = useApi<Project[]>("/api/v1/projects");
  const project = projects.data?.[0];
  const envs = useApi<Env[]>(project ? `/api/v1/projects/${project.id}/environments` : null);
  const keys = useApi<Key[]>(project ? `/api/v1/projects/${project.id}/api-keys` : null);
  const [envId, setEnvId] = useState("");
  const [newKey, setNewKey] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const ingest = process.env.NEXT_PUBLIC_INGEST_URL ?? "http://localhost:4318";

  async function createKey(e: FormEvent) {
    e.preventDefault();
    setError(null);
    try {
      const r = await api<{ key: string }>(`/api/v1/projects/${project!.id}/api-keys`, {
        method: "POST", body: JSON.stringify({ environmentId: envId || envs.data?.[0]?.id }),
      });
      setNewKey(r.key);
      void keys.reload();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not create key");
    }
  }

  async function revoke(id: string) {
    setError(null);
    try {
      await api(`/api/v1/projects/${project!.id}/api-keys/${id}`, { method: "DELETE" });
      void keys.reload();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not revoke key");
    }
  }

  if (projects.loading) return <Loading />;
  return (
    <main className="mx-auto max-w-3xl px-4 py-8 sm:px-6">
      <PageHeader title="Settings" />
      {(error || projects.error) && <div className="mt-4"><ErrorNote message={(error ?? projects.error)!} /></div>}

      <h2 className="mt-8 text-sm font-medium text-muted">Connect your application</h2>
      <Card className="mt-3 p-5">
        <p className="text-sm text-muted">
          Project <span className="text-fg">{project?.name}</span>. Create an API key, then install an SDK below.
        </p>
        <form onSubmit={createKey} className="mt-4 flex flex-wrap items-end gap-3">
          <label className="text-sm text-muted">Environment
            <select value={envId} onChange={(e) => setEnvId(e.target.value)} className={`${input} mt-1 w-48`}>
              {(envs.data ?? []).map((e) => <option key={e.id} value={e.id}>{e.name}</option>)}
            </select>
          </label>
          <button className={btn} disabled={!project}>Create API key</button>
        </form>
        {newKey && <p className="mt-4 rounded-md border border-accent-2/40 bg-accent-2/10 p-3 text-sm font-medium text-accent-2">Your key is filled into the snippets below. Copy it now; it will not be shown again.</p>}
      </Card>

      <h2 className="mt-8 text-sm font-medium text-muted">Install an SDK</h2>
      <Card className="mt-3 p-5"><SdkSnippets apiKey={newKey ?? "rx_your_api_key"} ingest={ingest} api={process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8000"} /></Card>

      <h2 className="mt-8 text-sm font-medium text-muted">API keys</h2>
      <Card className="mt-3 divide-y divide-line">
        {(keys.data ?? []).length === 0 ? <p className="p-4 text-sm text-muted">No keys yet.</p> : keys.data!.map((k) => (
          <div key={k.id} className="flex items-center justify-between gap-3 p-4 text-sm">
            <div><span className="font-mono">{k.prefix}…</span> <span className="text-muted">· {envs.data?.find((e) => e.id === k.environmentId)?.name ?? k.environmentId} · {timeAgo(k.createdAt)}</span></div>
            {k.status === "active" ? <button className={btnGhost} onClick={() => revoke(k.id)}>Revoke</button> : <span className="text-xs text-muted">revoked</span>}
          </div>
        ))}
      </Card>

      <h2 className="mt-8 text-sm font-medium text-muted">Webhooks</h2>
      <Card className="mt-3 p-5 text-sm text-muted">
        <p>Send deployments and alerts with the same API key to enrich incident analysis.</p>
        <pre className="mt-3 overflow-x-auto rounded bg-bg p-3 font-mono text-xs text-fg">{`curl -X POST ${process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8000"}/api/v1/webhooks/deployments \\
  -H "X-Resolvex-Key: <your key>" -H "Content-Type: application/json" \\
  -d '{"service":"payments","version":"v2.14.0"}'`}</pre>
      </Card>
    </main>
  );
}
