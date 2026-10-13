"use client";

import { useState, type FormEvent } from "react";
import { api, useApi, type Channel, type NotificationLog } from "@/lib/api";
import { timeAgo } from "@/lib/format";
import { btn, btnGhost, ErrorNote, input, Loading } from "./ui";

const EVENT_LABEL: Record<string, string> = { "incident.created": "Incident opened", "analysis.completed": "Cause found", "incident.resolved": "Resolved", test: "Test" };

export function NotificationsCard() {
  const channels = useApi<Channel[]>("/api/v1/notifications/channels");
  const log = useApi<NotificationLog[]>("/api/v1/notifications/log", 15000);
  const [kind, setKind] = useState<"slack" | "webhook">("slack");
  const [err, setErr] = useState<string | null>(null);
  const [secret, setSecret] = useState<string | null>(null);
  const [testMsg, setTestMsg] = useState<Record<string, string>>({});

  if (channels.error) return <ErrorNote message={channels.error} />;
  if (!channels.data) return <Loading />;

  async function add(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const f = new FormData(e.currentTarget);
    const form = e.currentTarget;
    setErr(null); setSecret(null);
    try {
      const r = await api<{ signingSecret: string }>("/api/v1/notifications/channels", { method: "POST", body: JSON.stringify({ kind, name: f.get("name"), url: f.get("url") }) });
      if (r.signingSecret) setSecret(r.signingSecret);
      form.reset();
      await channels.reload();
    } catch (e2) { setErr(e2 instanceof Error ? e2.message : "Could not add channel"); }
  }
  async function test(id: string) {
    setTestMsg((m) => ({ ...m, [id]: "Sending…" }));
    try {
      const r = await api<{ ok: boolean; error?: string }>(`/api/v1/notifications/channels/${id}/test`, { method: "POST" });
      setTestMsg((m) => ({ ...m, [id]: r.ok ? "Delivered ✓" : `Failed: ${r.error}` }));
      void log.reload();
    } catch (e) { setTestMsg((m) => ({ ...m, [id]: e instanceof Error ? e.message : "Failed" })); }
  }

  return (
    <div>
      <p className="text-sm text-muted">Get a message when an incident opens, when the probable cause is found, and when it is resolved.</p>
      {err && <div className="mt-3"><ErrorNote message={err} /></div>}
      {secret && <div className="mt-3 rounded-md border border-accent-2/40 bg-accent-2/10 p-3 text-sm"><p className="font-medium text-accent-2">Signing secret (shown once)</p><p className="mt-1 break-all font-mono text-xs">{secret}</p><p className="mt-1 text-xs text-muted">Each request carries <code>X-Resolvex-Signature: sha256=HMAC(body, secret)</code> so you can verify it came from Resolve-X.</p></div>}

      <ul className="mt-4 divide-y divide-line rounded-md border border-line">
        {channels.data.length === 0 && <li className="p-3 text-sm text-muted">No channels yet.</li>}
        {channels.data.map((c) => (
          <li key={c.id} className="flex flex-wrap items-center gap-3 p-3 text-sm">
            <span className="rounded bg-surface-2 px-2 py-0.5 text-xs">{c.kind}</span>
            <span className="min-w-0 flex-1 truncate">{c.name} <span className="font-mono text-xs text-muted">{c.urlHint}</span></span>
            {testMsg[c.id] && <span className="text-xs text-muted">{testMsg[c.id]}</span>}
            <button className={btnGhost} onClick={() => test(c.id)}>Send test</button>
            <button className={btnGhost} onClick={async () => { if (confirm(`Remove “${c.name}”?`)) { await api(`/api/v1/notifications/channels/${c.id}`, { method: "DELETE" }); await channels.reload(); } }}>Remove</button>
          </li>
        ))}
      </ul>

      <form onSubmit={add} className="mt-4 grid gap-3 sm:grid-cols-4">
        <label className="text-sm text-muted">Type<select value={kind} onChange={(e) => setKind(e.target.value as "slack" | "webhook")} className={`${input} mt-1`}><option value="slack">Slack</option><option value="webhook">Webhook</option></select></label>
        <label className="text-sm text-muted">Name<input name="name" required maxLength={80} placeholder="#oncall" className={`${input} mt-1`} /></label>
        <label className="text-sm text-muted sm:col-span-2">{kind === "slack" ? "Slack incoming-webhook URL" : "Endpoint URL (https)"}<input name="url" type="url" required placeholder={kind === "slack" ? "https://hooks.slack.com/services/…" : "https://example.com/hooks/resolvex"} className={`${input} mt-1 font-mono text-xs`} /></label>
        <div className="sm:col-span-4"><button className={btn}>Add channel</button></div>
      </form>

      {log.data && log.data.length > 0 && (
        <details className="mt-4 text-sm">
          <summary className="cursor-pointer text-muted">Recent deliveries</summary>
          <ul className="mt-2 space-y-1 font-mono text-xs">
            {log.data.slice(0, 10).map((l, i) => <li key={i} className="flex gap-2"><span className={l.ok ? "text-accent-2" : "text-danger"}>{l.ok ? "✓" : "✗"}</span><span>{EVENT_LABEL[l.event] ?? l.event}</span><span className="text-muted">{timeAgo(l.at)}{l.error ? ` · ${l.error}` : ""}</span></li>)}
          </ul>
        </details>
      )}
    </div>
  );
}
