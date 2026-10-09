"use client";

import { useMemo, useState } from "react";
import { api, useApi, type DashboardDef, type MetricName, type Widget } from "@/lib/api";
import { MetricChart } from "@/components/widgets";
import { btn, Card, Empty, ErrorNote, input, Loading, PageHeader, WindowSelect } from "@/components/ui";

const AGGS = [["avg", "Average"], ["sum", "Sum"], ["max", "Max"], ["min", "Min"], ["p50", "p50"], ["p95", "p95"], ["p99", "p99"], ["rate", "Rate / sec"], ["count", "Count"]];

export default function MetricsPage() {
  const [win, setWin] = useState("1h");
  const [search, setSearch] = useState("");
  const [sel, setSel] = useState<{ service: string; name: string } | null>(null);
  const [agg, setAgg] = useState("avg");
  const [groupBy, setGroupBy] = useState("");
  const [scope, setScope] = useState<"service" | "all">("service");
  const names = useApi<MetricName[]>(`/api/v1/metrics?window=${win}`, 30000);
  const labels = useApi<string[]>(sel ? `/api/v1/metrics/labels?name=${encodeURIComponent(sel.name)}&window=${win}` : null);
  const dashboards = useApi<DashboardDef[]>("/api/v1/dashboards");
  const [saveTo, setSaveTo] = useState("");
  const [msg, setMsg] = useState<string | null>(null);

  const list = useMemo(() => (names.data ?? []).filter((m) => (m.service + " " + m.name).toLowerCase().includes(search.toLowerCase())), [names.data, search]);
  const grouped = useMemo(() => {
    const g = new Map<string, MetricName[]>();
    list.forEach((m) => g.set(m.service, [...(g.get(m.service) ?? []), m]));
    return [...g];
  }, [list]);

  async function save() {
    if (!sel) return;
    const target = dashboards.data?.find((d) => d.id === saveTo);
    if (!target) return;
    const w: Widget = { id: crypto.randomUUID(), type: "metric", title: sel.name, span: 1, config: { metric: sel.name, service: scope === "service" ? sel.service : "", agg, groupBy } };
    try {
      await api(`/api/v1/dashboards/${target.id}`, { method: "PUT", body: JSON.stringify({ name: target.name, widgets: [...target.widgets, w] }) });
      setMsg(`Added to “${target.name}”.`);
    } catch (e) { setMsg(e instanceof Error ? e.message : "Could not save"); }
  }

  return (
    <main className="mx-auto max-w-6xl px-4 py-8 sm:px-6">
      <PageHeader title="Metrics explorer"><WindowSelect value={win} onChange={setWin} /></PageHeader>
      <div className="mt-6 grid gap-6 lg:grid-cols-[280px_1fr]">
        <Card className="max-h-[70vh] overflow-y-auto">
          <div className="sticky top-0 border-b border-line bg-surface p-3"><input aria-label="Search metrics" placeholder="Search metrics…" value={search} onChange={(e) => setSearch(e.target.value)} className={input} /></div>
          {names.loading && !names.data ? <Loading /> : names.error ? <div className="p-3"><ErrorNote message={names.error} /></div> : !grouped.length ? <Empty>No metrics yet. The Resolve-X SDKs export runtime and host metrics automatically.</Empty> : (
            grouped.map(([svc, ms]) => (
              <div key={svc}>
                <p className="bg-surface-2 px-3 py-1.5 text-xs font-medium text-muted">{svc}</p>
                <ul>{ms.map((m) => (
                  <li key={m.name}>
                    <button onClick={() => { setSel({ service: svc, name: m.name }); setGroupBy(""); setMsg(null); }}
                      className={`block w-full truncate px-3 py-1.5 text-left text-sm hover:bg-surface-2 ${sel?.name === m.name && sel.service === svc ? "bg-surface-2 text-accent" : ""}`}>{m.name}</button>
                  </li>))}
                </ul>
              </div>
            ))
          )}
        </Card>

        <section>
          {!sel ? <Card><Empty>Select a metric to chart it.</Empty></Card> : (
            <>
              <h2 className="text-lg font-medium"><span className="text-muted">{sel.service} ·</span> {sel.name}</h2>
              <div className="mt-3 flex flex-wrap items-end gap-3">
                <label className="text-xs text-muted">Aggregation<select value={agg} onChange={(e) => setAgg(e.target.value)} className={`${input} mt-1 w-36`}>{AGGS.map(([v, l]) => <option key={v} value={v}>{l}</option>)}</select></label>
                <label className="text-xs text-muted">Group by<select value={groupBy} onChange={(e) => setGroupBy(e.target.value)} className={`${input} mt-1 w-44`}>
                  <option value="">None</option><option value="service">service</option>{(labels.data ?? []).map((l) => <option key={l} value={l}>{l}</option>)}</select></label>
                <label className="text-xs text-muted">Scope<select value={scope} onChange={(e) => setScope(e.target.value as "service" | "all")} className={`${input} mt-1 w-40`}><option value="service">This service</option><option value="all">All services</option></select></label>
              </div>
              <Card className="mt-4 p-4"><MetricChart key={`${sel.service}${sel.name}${agg}${groupBy}${scope}${win}`} name={sel.name} service={scope === "service" ? sel.service : undefined} agg={agg} groupBy={groupBy || undefined} win={win} height={260} /></Card>
              <div className="mt-4 flex flex-wrap items-center gap-2">
                <select aria-label="Dashboard" value={saveTo} onChange={(e) => setSaveTo(e.target.value)} className={`${input} w-52`}>
                  <option value="">Save to dashboard…</option>{(dashboards.data ?? []).map((d) => <option key={d.id} value={d.id}>{d.name}</option>)}
                </select>
                <button className={btn} disabled={!saveTo} onClick={save}>Add widget</button>
                {!dashboards.data?.length && <span className="text-xs text-muted">Create a dashboard first.</span>}
                {msg && <span className="text-sm text-muted">{msg}</span>}
              </div>
            </>
          )}
        </section>
      </div>
    </main>
  );
}
