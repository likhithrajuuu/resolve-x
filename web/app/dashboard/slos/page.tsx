"use client";

import { useState, type FormEvent } from "react";
import { api, useApi, type ServiceRow, type SLO } from "@/lib/api";
import { Budget, sloTone } from "@/components/widgets";
import { btn, btnGhost, Card, Empty, ErrorNote, input, Loading, PageHeader } from "@/components/ui";

export default function SlosPage() {
  const { data, error, loading, reload } = useApi<SLO[]>("/api/v1/slos", 30000);
  const services = useApi<ServiceRow[]>("/api/v1/services?window=24h");
  const [creating, setCreating] = useState(false);
  const [kind, setKind] = useState<SLO["kind"]>("availability");
  const [err, setErr] = useState<string | null>(null);

  async function create(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const f = Object.fromEntries(new FormData(e.currentTarget)) as Record<string, string>;
    try {
      await api("/api/v1/slos", { method: "POST", body: JSON.stringify({ name: f.name, service: f.service, kind, objective: parseFloat(f.objective), latencyMs: kind === "latency" ? parseFloat(f.latencyMs) : undefined, windowDays: parseInt(f.windowDays) }) });
      setCreating(false); setErr(null); await reload();
    } catch (e2) { setErr(e2 instanceof Error ? e2.message : "Could not create SLO"); }
  }

  return (
    <main className="mx-auto max-w-5xl px-4 py-8 sm:px-6">
      <PageHeader title="Service level objectives"><button className={btn} onClick={() => setCreating((v) => !v)}>{creating ? "Cancel" : "New SLO"}</button></PageHeader>
      <p className="mt-2 text-sm text-muted">The error budget is how much failure your objective allows. Burn rate 1× spends it exactly over the window; above 1× you will run out early.</p>
      {creating && (
        <Card className="mt-6 p-4">
          <form onSubmit={create} className="grid gap-3 sm:grid-cols-3">
            {err && <div className="sm:col-span-3"><ErrorNote message={err} /></div>}
            <label className="text-sm text-muted sm:col-span-2">Name<input name="name" required className={`${input} mt-1`} placeholder="Checkout availability" /></label>
            <label className="text-sm text-muted">Service<select name="service" required className={`${input} mt-1`}><option value="">Choose…</option>{(services.data ?? []).map((s) => <option key={s.name}>{s.name}</option>)}</select></label>
            <label className="text-sm text-muted">Type<select value={kind} onChange={(e) => setKind(e.target.value as SLO["kind"])} className={`${input} mt-1`}><option value="availability">Availability (no errors)</option><option value="latency">Latency (fast enough)</option></select></label>
            <label className="text-sm text-muted">Objective (%)<input name="objective" type="number" step="any" min="1" max="99.999" defaultValue={99.9} required className={`${input} mt-1`} /></label>
            {kind === "latency" ? <label className="text-sm text-muted">Fast means ≤ (ms)<input name="latencyMs" type="number" min="1" step="any" defaultValue={500} className={`${input} mt-1`} /></label> : <span />}
            <label className="text-sm text-muted">Window<select name="windowDays" defaultValue="7" className={`${input} mt-1`}>{[1, 7, 14, 30].map((d) => <option key={d} value={d}>{d} days</option>)}</select></label>
            <div className="flex gap-2 sm:col-span-3"><button className={btn}>Create SLO</button><button type="button" className={btnGhost} onClick={() => setCreating(false)}>Cancel</button></div>
          </form>
        </Card>
      )}
      {error && <div className="mt-4"><ErrorNote message={error} /></div>}
      {loading && !data ? <Loading /> : !data?.length ? <Card className="mt-6"><Empty>No SLOs yet.</Empty></Card> : (
        <div className="mt-6 grid gap-4 md:grid-cols-2">
          {data.map((s) => (
            <Card key={s.id} className="p-5">
              <div className="flex items-start justify-between gap-3">
                <div><h2 className="font-medium">{s.name}</h2><p className="text-xs text-muted">{s.service} · {s.kind === "latency" ? `≤ ${s.latencyMs} ms` : "no errors"} · {s.windowDays}d</p></div>
                <span className={`text-xs font-medium uppercase ${sloTone[s.status]}`}>{s.status === "nodata" ? "no data" : s.status}</span>
              </div>
              {s.status === "nodata" ? <p className="mt-4 text-sm text-muted">No traffic in this window yet.</p> : (<>
                <p className="mt-4 text-3xl font-semibold">{s.sli.toFixed(3)}%<span className="ml-2 text-sm font-normal text-muted">of {s.objective}% target</span></p>
                <Budget remaining={s.budgetRemaining} status={s.status} />
                <dl className="mt-3 grid grid-cols-3 gap-2 text-xs">
                  <div><dt className="text-muted">Budget left</dt><dd className={s.budgetRemaining < 0 ? "text-danger" : ""}>{s.budgetRemaining.toFixed(1)}%</dd></div>
                  <div><dt className="text-muted">Burn rate (1h)</dt><dd className={s.burnRate1h >= 2 ? "text-warn" : ""}>{s.burnRate1h.toFixed(1)}×</dd></div>
                  <div><dt className="text-muted">Bad / total</dt><dd>{s.bad.toLocaleString()} / {s.total.toLocaleString()}</dd></div>
                </dl>
              </>)}
              <button className="mt-4 text-xs text-muted hover:text-danger" onClick={async () => { if (confirm(`Delete “${s.name}”?`)) { await api(`/api/v1/slos/${s.id}`, { method: "DELETE" }); await reload(); } }}>Delete</button>
            </Card>
          ))}
        </div>
      )}
    </main>
  );
}
