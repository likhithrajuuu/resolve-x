"use client";

import { useState, type FormEvent } from "react";
import { api, useApi, type AiSettings } from "@/lib/api";
import { btn, btnGhost, ErrorNote, input, Loading } from "./ui";

export function AiSettingsCard() {
  const { data, error, reload } = useApi<AiSettings>("/api/v1/ai/settings");
  const [provider, setProvider] = useState<string | null>(null);
  const [msg, setMsg] = useState<{ ok: boolean; text: string } | null>(null);
  const [busy, setBusy] = useState(false);

  if (error) return <ErrorNote message={error} />;
  if (!data) return <Loading />;
  const current = provider ?? (data.source === "none" ? "anthropic" : data.provider);
  const info = data.providers.find((p) => p.id === current)!;
  const sameAsSaved = data.source !== "none" && current === data.provider;

  async function save(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const f = new FormData(e.currentTarget);
    setBusy(true); setMsg(null);
    try {
      await api("/api/v1/ai/settings", { method: "PUT", body: JSON.stringify({ provider: current, model: f.get("model"), baseUrl: f.get("baseUrl") ?? "", apiKey: f.get("apiKey") ?? "" }) });
      (e.target as HTMLFormElement).reset();
      setMsg({ ok: true, text: "Saved. Use “Test connection” to check the key." });
      await reload();
    } catch (err) { setMsg({ ok: false, text: err instanceof Error ? err.message : "Could not save" }); }
    setBusy(false);
  }

  async function test() {
    setBusy(true); setMsg(null);
    try {
      const r = await api<{ ok: boolean; error?: string; ms?: number; model?: string }>("/api/v1/ai/test", { method: "POST" });
      setMsg(r.ok ? { ok: true, text: `Connected to ${r.model} in ${r.ms} ms.` } : { ok: false, text: r.error ?? "Connection failed" });
    } catch (err) { setMsg({ ok: false, text: err instanceof Error ? err.message : "Test failed" }); }
    setBusy(false);
  }

  async function remove() {
    if (!confirm("Remove the AI provider and its stored key?")) return;
    await api("/api/v1/ai/settings", { method: "DELETE" });
    setProvider(null); setMsg(null); await reload();
  }

  return (
    <div id="ai">
      {data.source === "env" && <p className="mb-4 rounded-md border border-line bg-surface-2 p-3 text-sm text-muted">This installation has a server-wide default ({data.provider}). Saving your own settings below overrides it for your organization.</p>}
      {!data.canEdit && <p className="mb-4 text-sm text-warn">Only owners and admins can change these settings.</p>}
      <fieldset disabled={!data.canEdit || busy} className="space-y-4">
        <legend className="sr-only">AI provider</legend>
        <div role="radiogroup" aria-label="Provider" className="grid gap-2 sm:grid-cols-2">
          {data.providers.map((p) => (
            <button type="button" key={p.id} role="radio" aria-checked={current === p.id} onClick={() => setProvider(p.id)}
              className={`rounded-lg border p-3 text-left text-sm transition-colors ${current === p.id ? "border-accent bg-accent/10" : "border-line hover:bg-surface-2"}`}>
              <span className="font-medium">{p.label}</span>
            </button>
          ))}
        </div>
        <p className="text-xs text-muted">{info.help}</p>

        <form onSubmit={save} className="grid gap-3 sm:grid-cols-2" key={current}>
          <label className="text-sm text-muted">Model
            <input name="model" list="models" defaultValue={sameAsSaved ? data.model : info.defaultModel} placeholder={info.defaultModel || "model name"} className={`${input} mt-1`} />
            <datalist id="models">{(info.models ?? []).map((m) => <option key={m} value={m} />)}</datalist>
          </label>
          {info.needsBaseUrl
            ? <label className="text-sm text-muted">Base URL<input name="baseUrl" required defaultValue={sameAsSaved ? data.baseUrl : ""} placeholder="https://api.example.com/v1" className={`${input} mt-1`} /></label>
            : <label className="text-sm text-muted">Base URL <span className="text-muted/60">(optional proxy)</span><input name="baseUrl" defaultValue={sameAsSaved ? data.baseUrl : ""} placeholder="leave empty for the default" className={`${input} mt-1`} /></label>}
          <label className="text-sm text-muted sm:col-span-2">API key {info.keyOptional && <span className="text-muted/60">(optional for local servers)</span>}
            <input name="apiKey" type="password" autoComplete="off" spellCheck={false} className={`${input} mt-1 font-mono`}
              placeholder={sameAsSaved && data.keyHint ? `saved ••••${data.keyHint} — type a new key to replace it` : "paste your API key"} />
          </label>
          <div className="flex flex-wrap items-center gap-2 sm:col-span-2">
            <button className={btn}>Save</button>
            <button type="button" className={btnGhost} onClick={test} disabled={!data.configured}>Test connection</button>
            {data.source === "tenant" && <button type="button" className={btnGhost} onClick={remove}>Remove</button>}
          </div>
        </form>
      </fieldset>
      {msg && <p role="status" className={`mt-3 text-sm ${msg.ok ? "text-accent-2" : "text-danger"}`}>{msg.text}</p>}
      <p className="mt-4 text-xs text-muted">Your key is encrypted at rest and never shown again. The assistant and incident analysis send relevant telemetry (including log text) to the provider you choose.</p>
    </div>
  );
}
