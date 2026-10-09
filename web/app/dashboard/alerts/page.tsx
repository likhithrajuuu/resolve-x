"use client";

import { useState, type FormEvent } from "react";
import { api, useApi, type MetricName, type Rule, type ServiceRow } from "@/lib/api";
import { fmtMs, timeAgo } from "@/lib/format";
import { btn, btnGhost, Card, Empty, ErrorNote, input, Loading, PageHeader, SeverityBadge } from "@/components/ui";

const stateTone = { firing: "bg-danger/15 text-danger", ok: "bg-accent-2/15 text-accent-2", nodata: "bg-surface-2 text-muted", unknown: "bg-surface-2 text-muted" } as const;

function describe(r: Rule) {
  if (r.kind === "error_rate") return `error rate of ${r.service} ${r.op} ${(r.threshold * 100).toFixed(1)}%`;
  if (r.kind === "latency_p95") return `p95 latency of ${r.service} ${r.op} ${fmtMs(r.threshold)}`;
  return `${r.agg}(${r.metric})${r.service ? ` on ${r.service}` : ""} ${r.op} ${r.threshold}`;
}
const fmtValue = (r: Rule) => (r.lastValue == null ? "–" : r.kind === "error_rate" ? `${(r.lastValue * 100).toFixed(2)}%` : r.kind === "latency_p95" ? fmtMs(r.lastValue) : String(+r.lastValue.toFixed(4)));

export default function AlertsPage() {
  const { data, error, loading, reload } = useApi<Rule[]>("/api/v1/alert-rules", 10000);
  const [creating, setCreating] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  async function act(fn: () => Promise<unknown>) {
    setErr(null);
    try { await fn(); await reload(); } catch (e) { setErr(e instanceof Error ? e.message : "Failed"); }
  }

  return (
    <main className="mx-auto max-w-5xl px-4 py-8 sm:px-6">
      <PageHeader title="Alert rules"><button className={btn} onClick={() => setCreating((v) => !v)}>{creating ? "Cancel" : "New rule"}</button></PageHeader>
      <p className="mt-2 text-sm text-muted">A firing rule opens an incident, which Resolve-X then analyses. Rules are evaluated every 30 seconds.</p>
      {creating && <NewRule onDone={() => { setCreating(false); void reload(); }} />}
      {err && <div className="mt-4"><ErrorNote message={err} /></div>}
      <Card className="mt-6 divide-y divide-line">
        {error && <div className="p-4"><ErrorNote message={error} /></div>}
        {loading && !data ? <Loading /> : !data?.length ? <Empty>No rules. Resolve-X still opens incidents automatically on error-rate spikes and latency jumps; add rules for your own thresholds.</Empty> : data.map((r) => (
          <div key={r.id} className="flex flex-wrap items-center gap-3 p-4">
            <span className={`w-20 rounded px-2 py-0.5 text-center text-xs font-medium ${stateTone[r.lastState]}`}>{r.enabled ? r.lastState : "paused"}</span>
            <div className="min-w-0 flex-1">
              <p className="truncate font-medium">{r.name} <SeverityBadge s={r.severity} /></p>
              <p className="text-xs text-muted">When {describe(r)} over {r.windowMinutes} min · now {fmtValue(r)}{r.lastEvalAt ? ` · checked ${timeAgo(r.lastEvalAt)}` : ""}</p>
            </div>
            <button className={btnGhost} onClick={() => act(() => api(`/api/v1/alert-rules/${r.id}`, { method: "PATCH", body: JSON.stringify({ enabled: !r.enabled }) }))}>{r.enabled ? "Pause" : "Resume"}</button>
            <button className={btnGhost} onClick={() => confirm(`Delete “${r.name}”?`) && act(() => api(`/api/v1/alert-rules/${r.id}`, { method: "DELETE" }))}>Delete</button>
          </div>
        ))}
      </Card>
    </main>
  );
}

function NewRule({ onDone }: { onDone: () => void }) {
  const [kind, setKind] = useState<Rule["kind"]>("error_rate");
  const [error, setError] = useState<string | null>(null);
  const services = useApi<ServiceRow[]>("/api/v1/services?window=24h");
  const metrics = useApi<MetricName[]>(kind === "metric" ? "/api/v1/metrics?window=24h" : null);

  async function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const f = Object.fromEntries(new FormData(e.currentTarget)) as Record<string, string>;
    let threshold = parseFloat(f.threshold);
    if (kind === "error_rate") threshold /= 100; // the form takes a percentage
    try {
      await api("/api/v1/alert-rules", { method: "POST", body: JSON.stringify({ name: f.name, kind, service: f.service ?? "", metric: f.metric ?? "", agg: f.agg ?? "avg", op: f.op ?? ">", threshold, windowMinutes: parseInt(f.windowMinutes), severity: f.severity }) });
      onDone();
    } catch (err) { setError(err instanceof Error ? err.message : "Could not create rule"); }
  }

  return (
    <Card className="mt-6 p-4">
      <form onSubmit={submit} className="grid gap-3 sm:grid-cols-3">
        {error && <div className="sm:col-span-3"><ErrorNote message={error} /></div>}
        <label className="text-sm text-muted sm:col-span-2">Name<input name="name" required maxLength={120} className={`${input} mt-1`} placeholder="Payments errors too high" /></label>
        <label className="text-sm text-muted">Type<select value={kind} onChange={(e) => setKind(e.target.value as Rule["kind"])} className={`${input} mt-1`}>
          <option value="error_rate">Service error rate</option><option value="latency_p95">Service p95 latency</option><option value="metric">Metric threshold</option></select></label>
        <label className="text-sm text-muted">Service<select name="service" required={kind !== "metric"} className={`${input} mt-1`}><option value="">{kind === "metric" ? "Any" : "Choose…"}</option>{(services.data ?? []).map((s) => <option key={s.name}>{s.name}</option>)}</select></label>
        {kind === "metric" && (<>
          <label className="text-sm text-muted">Metric<select name="metric" required className={`${input} mt-1`}><option value="">Choose…</option>{[...new Set((metrics.data ?? []).map((m) => m.name))].map((n) => <option key={n}>{n}</option>)}</select></label>
          <label className="text-sm text-muted">Aggregation<select name="agg" className={`${input} mt-1`}>{["avg", "max", "min", "sum", "p95"].map((a) => <option key={a}>{a}</option>)}</select></label>
        </>)}
        {kind === "metric" && <label className="text-sm text-muted">Condition<select name="op" className={`${input} mt-1`}><option value=">">is above</option><option value="<">is below</option></select></label>}
        <label className="text-sm text-muted">Threshold {kind === "error_rate" ? "(%)" : kind === "latency_p95" ? "(ms)" : ""}<input name="threshold" type="number" step="any" min="0" required className={`${input} mt-1`} /></label>
        <label className="text-sm text-muted">Window (minutes)<input name="windowMinutes" type="number" min="1" max="1440" defaultValue={5} className={`${input} mt-1`} /></label>
        <label className="text-sm text-muted">Severity<select name="severity" defaultValue="SEV-2" className={`${input} mt-1`}>{["SEV-1", "SEV-2", "SEV-3"].map((s) => <option key={s}>{s}</option>)}</select></label>
        <div className="flex gap-2 sm:col-span-3"><button className={btn}>Create rule</button><button type="button" className={btnGhost} onClick={onDone}>Cancel</button></div>
      </form>
    </Card>
  );
}
