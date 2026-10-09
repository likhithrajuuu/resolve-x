"use client";

import Link from "next/link";
import { useParams, useRouter } from "next/navigation";
import { useState } from "react";
import { api, useApi, type DashboardDef, type MetricName, type Widget } from "@/lib/api";
import { WIDGET_TYPES, WidgetView } from "@/components/widgets";
import { btn, btnGhost, Card, ErrorNote, input, Loading, WindowSelect } from "@/components/ui";

export default function DashboardView() {
  const { id } = useParams<{ id: string }>();
  const router = useRouter();
  const { data, error, loading, reload } = useApi<DashboardDef>(`/api/v1/dashboards/${id}`);
  const [win, setWin] = useState("1h");
  const [edit, setEdit] = useState(false);
  const [draft, setDraft] = useState<Widget[] | null>(null);
  const [name, setName] = useState("");
  const [err, setErr] = useState<string | null>(null);
  const [adding, setAdding] = useState(false);

  if (loading && !data) return <Loading />;
  if (!data) return <main className="p-6"><ErrorNote message={error ?? "Not found"} /></main>;
  const widgets = draft ?? data.widgets;

  const startEdit = () => { setDraft(data.widgets); setName(data.name); setEdit(true); };
  async function save() {
    try {
      await api(`/api/v1/dashboards/${id}`, { method: "PUT", body: JSON.stringify({ name, widgets: draft }) });
      setEdit(false); setDraft(null); await reload();
    } catch (e) { setErr(e instanceof Error ? e.message : "Could not save"); }
  }
  async function remove() {
    if (!confirm(`Delete “${data!.name}”?`)) return;
    await api(`/api/v1/dashboards/${id}`, { method: "DELETE" });
    router.replace("/dashboard/dashboards");
  }
  const move = (i: number, d: number) => setDraft((w) => { const a = [...(w ?? [])]; const j = i + d; if (j < 0 || j >= a.length) return a; [a[i], a[j]] = [a[j], a[i]]; return a; });

  return (
    <main className="mx-auto max-w-6xl px-4 py-8 sm:px-6">
      <Link href="/dashboard/dashboards" className="text-sm text-muted hover:text-fg">← Dashboards</Link>
      <div className="mt-3 flex flex-wrap items-center justify-between gap-3">
        {edit ? <input aria-label="Dashboard name" value={name} onChange={(e) => setName(e.target.value)} className={`${input} max-w-sm text-lg`} /> : <h1 className="text-2xl font-semibold tracking-tight">{data.name}</h1>}
        <div className="flex flex-wrap items-center gap-2">
          <WindowSelect value={win} onChange={setWin} />
          {edit ? (<><button className={btn} onClick={save}>Save</button><button className={btnGhost} onClick={() => { setEdit(false); setDraft(null); }}>Cancel</button></>) : (<><button className={btnGhost} onClick={startEdit}>Edit</button><button className={btnGhost} onClick={remove}>Delete</button></>)}
        </div>
      </div>
      {err && <div className="mt-4"><ErrorNote message={err} /></div>}

      {widgets.length === 0 && <Card className="mt-6 p-8 text-center text-sm text-muted">This dashboard is empty. Click Edit to add widgets.</Card>}
      <div className="mt-6 grid gap-4 lg:grid-cols-2">
        {widgets.map((w, i) => (
          <Card key={w.id} className={`p-4 ${w.span === 2 ? "lg:col-span-2" : ""}`}>
            <div className="mb-3 flex items-center justify-between gap-2">
              {edit ? <input aria-label="Widget title" value={w.title} onChange={(e) => setDraft(widgets.map((x) => (x.id === w.id ? { ...x, title: e.target.value } : x)))} className={`${input} max-w-xs py-1`} /> : <h2 className="truncate text-sm font-medium">{w.title}</h2>}
              {edit && (
                <div className="flex shrink-0 gap-1 text-xs">
                  <button className={btnGhost} aria-label="Move up" onClick={() => move(i, -1)}>↑</button>
                  <button className={btnGhost} aria-label="Move down" onClick={() => move(i, 1)}>↓</button>
                  <button className={btnGhost} onClick={() => setDraft(widgets.map((x) => (x.id === w.id ? { ...x, span: x.span === 2 ? 1 : 2 } : x)))}>{w.span === 2 ? "Narrow" : "Wide"}</button>
                  <button className={btnGhost} onClick={() => setDraft(widgets.filter((x) => x.id !== w.id))}>Remove</button>
                </div>
              )}
            </div>
            <WidgetView w={w} win={win} />
          </Card>
        ))}
      </div>
      {edit && (adding ? <AddWidget onAdd={(w) => { setDraft([...widgets, w]); setAdding(false); }} onCancel={() => setAdding(false)} /> : <button className={`${btn} mt-4`} onClick={() => setAdding(true)}>Add widget</button>)}
    </main>
  );
}

function AddWidget({ onAdd, onCancel }: { onAdd: (w: Widget) => void; onCancel: () => void }) {
  const [type, setType] = useState<Widget["type"]>("service");
  const [cfg, setCfg] = useState<Record<string, string>>({ agg: "avg" });
  const services = useApi<{ name: string }[]>("/api/v1/services?window=24h");
  const metrics = useApi<MetricName[]>(type === "metric" ? "/api/v1/metrics?window=24h" : null);
  const set = (k: string, v: string) => setCfg((c) => ({ ...c, [k]: v }));
  const meta = WIDGET_TYPES.find((t) => t.type === type)!;
  const ready = type === "service" ? !!cfg.service : type === "metric" ? !!cfg.metric : true;
  const title = type === "metric" ? cfg.metric : type === "service" ? `${cfg.service} health` : meta.label;

  return (
    <Card className="mt-4 p-4">
      <h2 className="text-sm font-medium">Add widget</h2>
      <div className="mt-3 flex flex-wrap items-end gap-3">
        <label className="text-xs text-muted">Type<select value={type} onChange={(e) => { setType(e.target.value as Widget["type"]); setCfg({ agg: "avg" }); }} className={`${input} mt-1 w-52`}>{WIDGET_TYPES.map((t) => <option key={t.type} value={t.type}>{t.label}</option>)}</select></label>
        {(type === "service" || type === "logs" || type === "metric") && (
          <label className="text-xs text-muted">Service<select value={cfg.service ?? ""} onChange={(e) => set("service", e.target.value)} className={`${input} mt-1 w-44`}>
            <option value="">{type === "service" ? "Choose…" : "All"}</option>{(services.data ?? []).map((s) => <option key={s.name}>{s.name}</option>)}</select></label>
        )}
        {type === "metric" && (<>
          <label className="text-xs text-muted">Metric<select value={cfg.metric ?? ""} onChange={(e) => set("metric", e.target.value)} className={`${input} mt-1 w-56`}>
            <option value="">Choose…</option>{[...new Set((metrics.data ?? []).filter((m) => !cfg.service || m.service === cfg.service).map((m) => m.name))].map((n) => <option key={n}>{n}</option>)}</select></label>
          <label className="text-xs text-muted">Aggregation<select value={cfg.agg} onChange={(e) => set("agg", e.target.value)} className={`${input} mt-1 w-28`}>{["avg", "sum", "max", "min", "p95", "rate"].map((a) => <option key={a}>{a}</option>)}</select></label>
        </>)}
        <button className={btn} disabled={!ready} onClick={() => onAdd({ id: crypto.randomUUID(), type, title, span: type === "service" ? 2 : 1, config: cfg })}>Add</button>
        <button className={btnGhost} onClick={onCancel}>Cancel</button>
      </div>
      <p className="mt-2 text-xs text-muted">{meta.help}</p>
    </Card>
  );
}
