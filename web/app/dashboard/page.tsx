"use client";

import Link from "next/link";
import { useState, type FormEvent } from "react";
import { api, useApi, type Incident, type ServiceRow } from "@/lib/api";
import { fmtNum, fmtPct, timeAgo } from "@/lib/format";
import { btn, btnGhost, Card, Empty, ErrorNote, input, Loading, PageHeader, SeverityBadge, StatusLabel } from "@/components/ui";

export default function IncidentsPage() {
  const [filter, setFilter] = useState("");
  const incidents = useApi<Incident[]>(`/api/v1/incidents${filter ? `?status=${filter}` : ""}`, 5000);
  const services = useApi<ServiceRow[]>("/api/v1/services?window=1h", 15000);
  const [creating, setCreating] = useState(false);

  const all = incidents.data ?? [];
  const open = all.filter((i) => i.status !== "resolved").length;
  const spans = (services.data ?? []).reduce((a, s) => a + s.spans, 0);
  const errs = (services.data ?? []).reduce((a, s) => a + s.errors, 0);

  const kpis = [
    ["Open incidents", incidents.data ? String(open) : "–"],
    ["Services seen (1h)", services.data ? String(services.data.length) : "–"],
    ["Entry spans (1h)", services.data ? fmtNum(spans) : "–"],
    ["Error rate (1h)", services.data ? fmtPct(spans ? errs / spans : 0) : "–"],
  ];

  return (
    <main className="mx-auto max-w-5xl px-4 py-8 sm:px-6">
      <PageHeader title="Incidents">
        <select aria-label="Filter by status" value={filter} onChange={(e) => setFilter(e.target.value)} className="rounded-md border border-line bg-surface px-2.5 py-2 text-sm">
          <option value="">All</option>
          {["open", "investigating", "identified", "resolved"].map((s) => <option key={s} value={s}>{s}</option>)}
        </select>
        <button className={btn} onClick={() => setCreating((v) => !v)}>{creating ? "Cancel" : "New incident"}</button>
      </PageHeader>

      <dl className="mt-6 grid grid-cols-2 gap-3 lg:grid-cols-4">
        {kpis.map(([label, value]) => (
          <Card key={label} className="p-4">
            <dt className="text-xs text-muted">{label}</dt>
            <dd className="mt-1 text-xl font-semibold">{value}</dd>
          </Card>
        ))}
      </dl>

      {services.data?.length === 0 && incidents.data?.length === 0 && (
        <Card className="mt-6 border-accent/40 p-5">
          <h2 className="font-medium">Connect your first application</h2>
          <p className="mt-1 text-sm text-muted">No telemetry has arrived yet. Create an API key and point an OpenTelemetry SDK at Resolve-X; services and incidents will appear here.</p>
          <Link href="/dashboard/settings" className={`${btn} mt-4 inline-block`}>Set up an API key</Link>
        </Card>
      )}

      {creating && <NewIncident onDone={() => { setCreating(false); void incidents.reload(); }} />}

      <Card className="mt-8 overflow-hidden">
        {incidents.error && <div className="p-4"><ErrorNote message={incidents.error} /></div>}
        {incidents.loading && !incidents.data ? <Loading /> : all.length === 0 ? (
          <Empty>
            No incidents. Resolve-X opens one automatically when error rate or latency spikes, or you can{" "}
            <button className="text-accent underline" onClick={() => setCreating(true)}>create one</button>.
          </Empty>
        ) : (
          <ul className="divide-y divide-line">
            {all.map((i) => (
              <li key={i.id}>
                <Link href={`/dashboard/incidents/${i.id}`} className="flex flex-col gap-2 p-4 transition-colors hover:bg-surface-2 sm:flex-row sm:items-center sm:gap-4">
                  <SeverityBadge s={i.severity} />
                  <div className="min-w-0 flex-1">
                    <p className="truncate font-medium">{i.title}</p>
                    <p className="mt-0.5 truncate text-sm text-muted">{i.service || "no service"} · {i.source}</p>
                  </div>
                  <div className="text-sm sm:text-right">
                    <StatusLabel s={i.status} />
                    <p className="text-xs text-muted">{timeAgo(i.createdAt)}</p>
                  </div>
                </Link>
              </li>
            ))}
          </ul>
        )}
      </Card>
    </main>
  );
}

function NewIncident({ onDone }: { onDone: () => void }) {
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  async function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setBusy(true);
    const f = new FormData(e.currentTarget);
    try {
      await api("/api/v1/incidents", { method: "POST", body: JSON.stringify(Object.fromEntries(f)) });
      onDone();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed");
      setBusy(false);
    }
  }
  return (
    <Card className="mt-6 p-4">
      <form onSubmit={submit} className="grid gap-3 sm:grid-cols-4">
        {error && <div className="sm:col-span-4"><ErrorNote message={error} /></div>}
        <label className="sm:col-span-2 text-sm text-muted">Title
          <input name="title" required maxLength={200} className={`${input} mt-1`} />
        </label>
        <label className="text-sm text-muted">Service
          <input name="service" placeholder="optional" className={`${input} mt-1`} />
        </label>
        <label className="text-sm text-muted">Severity
          <select name="severity" defaultValue="SEV-2" className={`${input} mt-1`}>
            {["SEV-1", "SEV-2", "SEV-3"].map((s) => <option key={s}>{s}</option>)}
          </select>
        </label>
        <label className="sm:col-span-4 text-sm text-muted">Description
          <textarea name="description" rows={2} className={`${input} mt-1`} />
        </label>
        <div className="sm:col-span-4 flex gap-2">
          <button disabled={busy} className={btn}>Open incident</button>
          <button type="button" className={btnGhost} onClick={onDone}>Cancel</button>
        </div>
      </form>
    </Card>
  );
}
