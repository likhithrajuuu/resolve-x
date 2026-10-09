"use client";

import Link from "next/link";
import { useApi, type Incident, type LogVolume, type MetricSeries, type Point, type SLO, type Widget } from "@/lib/api";
import { fmtMs, fmtNum } from "@/lib/format";
import { Legend, LineChart, StackedBars, type Line } from "./charts";
import { ErrorNote, Loading, SeverityBadge, StatusLabel } from "./ui";

export const WIDGET_TYPES: { type: Widget["type"]; label: string; help: string }[] = [
  { type: "metric", label: "Metric time series", help: "Any metric, any aggregation, optionally grouped by an attribute." },
  { type: "service", label: "Service health (RED)", help: "Requests, error rate and p95 latency for one service." },
  { type: "logs", label: "Log volume", help: "Errors, warnings and info over time, optionally for one service." },
  { type: "incidents", label: "Active incidents", help: "Unresolved incidents, most recent first." },
  { type: "slo", label: "SLO status", help: "Error-budget status for every SLO." },
];

const toLines = (series: MetricSeries[]): Line[] =>
  series.map((s) => ({ name: s.group, points: s.points.map((p) => ({ t: +new Date(p.t), v: p.value })) }));

export function metricFormat(name: string, agg: string) {
  if (agg === "rate" || agg === "count") return (v: number) => fmtNum(Math.round(v * 100) / 100);
  if (/duration|latency/i.test(name)) return (v: number) => (v < 1 ? `${(v * 1000).toFixed(1)} ms` : `${v.toFixed(2)} s`);
  if (/bytes|memory/i.test(name)) return (v: number) => (v >= 1e9 ? `${(v / 1e9).toFixed(1)} GB` : v >= 1e6 ? `${(v / 1e6).toFixed(0)} MB` : v >= 1e3 ? `${(v / 1e3).toFixed(0)} KB` : `${v.toFixed(0)} B`);
  if (/utili[sz]ation|ratio/i.test(name)) return (v: number) => `${(v * 100).toFixed(1)}%`;
  return (v: number) => (Math.abs(v) >= 1000 ? fmtNum(Math.round(v)) : String(+v.toFixed(3)));
}

export function MetricChart({ name, service, agg, groupBy, win, height = 200 }: { name: string; service?: string; agg: string; groupBy?: string; win: string; height?: number }) {
  const qs = new URLSearchParams({ name, agg, window: win });
  if (service) qs.set("service", service);
  if (groupBy) qs.set("groupBy", groupBy);
  const { data, error, loading } = useApi<MetricSeries[]>(`/api/v1/metrics/series?${qs}`, 15000);
  if (error) return <ErrorNote message={error} />;
  if (loading && !data) return <Loading />;
  const lines = toLines(data ?? []);
  return <><LineChart lines={lines} height={height} area={lines.length === 1} format={metricFormat(name, agg)} /><Legend lines={lines} /></>;
}

export function ServiceRed({ service, win }: { service: string; win: string }) {
  const { data, error, loading } = useApi<Point[]>(`/api/v1/services/${encodeURIComponent(service)}/timeseries?window=${win}`, 15000);
  if (error) return <ErrorNote message={error} />;
  if (loading && !data) return <Loading />;
  const pts = data ?? [];
  const mk = (f: (p: Point) => number): Line[] => [{ name: "", points: pts.map((p) => ({ t: +new Date(p.t), v: f(p) })) }];
  return (
    <div className="grid gap-4 sm:grid-cols-3">
      <div><p className="mb-1 text-xs text-muted">Requests</p><LineChart lines={mk((p) => p.count)} height={110} area format={(v) => fmtNum(Math.round(v))} /></div>
      <div><p className="mb-1 text-xs text-muted">Error rate</p><LineChart lines={[{ ...mk((p) => (p.count ? p.errors / p.count : 0))[0], color: "var(--danger)" }]} height={110} area format={(v) => `${(v * 100).toFixed(1)}%`} /></div>
      <div><p className="mb-1 text-xs text-muted">p95 latency</p><LineChart lines={[{ ...mk((p) => p.p95Ms)[0], color: "var(--warn)" }]} height={110} area format={fmtMs} /></div>
    </div>
  );
}

export function LogVolumeChart({ service, win, height = 120 }: { service?: string; win: string; height?: number }) {
  const qs = new URLSearchParams({ window: win });
  if (service) qs.set("service", service);
  const { data, error, loading } = useApi<LogVolume[]>(`/api/v1/logs/volume?${qs}`, 10000);
  if (error) return <ErrorNote message={error} />;
  if (loading && !data) return <Loading />;
  return (
    <>
      <StackedBars height={height} data={(data ?? []).map((d) => ({ t: +new Date(d.t), parts: [
        { value: d.info, color: "var(--line)", label: "info" }, { value: d.warn, color: "var(--warn)", label: "warn" }, { value: d.error, color: "var(--danger)", label: "error" }] }))} />
      <p className="mt-1 flex gap-3 text-xs text-muted"><span style={{ color: "var(--danger)" }}>■ error</span><span style={{ color: "var(--warn)" }}>■ warn</span><span>■ info</span></p>
    </>
  );
}

export function IncidentList({ limit = 5 }: { limit?: number }) {
  const { data, error, loading } = useApi<Incident[]>("/api/v1/incidents", 8000);
  if (error) return <ErrorNote message={error} />;
  if (loading && !data) return <Loading />;
  const open = (data ?? []).filter((i) => i.status !== "resolved").slice(0, limit);
  if (!open.length) return <p className="py-6 text-center text-sm text-accent-2">No active incidents.</p>;
  return (
    <ul className="divide-y divide-line">
      {open.map((i) => (
        <li key={i.id}>
          <Link href={`/dashboard/incidents/${i.id}`} className="flex items-center gap-3 py-2.5 text-sm hover:text-accent">
            <SeverityBadge s={i.severity} /><span className="min-w-0 flex-1 truncate">{i.title}</span><StatusLabel s={i.status} />
          </Link>
        </li>
      ))}
    </ul>
  );
}

export const sloTone = { ok: "text-accent-2", warning: "text-warn", breached: "text-danger", nodata: "text-muted" } as const;

export function SloList() {
  const { data, error, loading } = useApi<SLO[]>("/api/v1/slos", 30000);
  if (error) return <ErrorNote message={error} />;
  if (loading && !data) return <Loading />;
  if (!data?.length) return <p className="py-6 text-center text-sm text-muted">No SLOs yet. <Link className="text-accent" href="/dashboard/slos">Create one</Link>.</p>;
  return (
    <ul className="space-y-3">
      {data.map((s) => (
        <li key={s.id} className="text-sm">
          <div className="flex justify-between"><span className="truncate">{s.name}</span><span className={sloTone[s.status]}>{s.status === "nodata" ? "no data" : `${s.sli.toFixed(2)}% / ${s.objective}%`}</span></div>
          <Budget remaining={s.budgetRemaining} status={s.status} />
        </li>
      ))}
    </ul>
  );
}

export function Budget({ remaining, status }: { remaining: number; status: SLO["status"] }) {
  const pct = Math.max(0, Math.min(100, remaining));
  const color = status === "breached" ? "var(--danger)" : status === "warning" ? "var(--warn)" : "var(--accent-2)";
  return <div className="mt-1 h-1.5 overflow-hidden rounded bg-surface-2" role="meter" aria-valuenow={Math.round(remaining)} aria-valuemin={0} aria-valuemax={100} aria-label="Error budget remaining"><div className="h-full" style={{ width: `${pct}%`, background: color }} /></div>;
}

export function WidgetView({ w, win }: { w: Widget; win: string }) {
  switch (w.type) {
    case "metric": return <MetricChart name={w.config.metric} service={w.config.service || undefined} agg={w.config.agg || "avg"} groupBy={w.config.groupBy || undefined} win={win} />;
    case "service": return <ServiceRed service={w.config.service} win={win} />;
    case "logs": return <LogVolumeChart service={w.config.service || undefined} win={win} />;
    case "incidents": return <IncidentList />;
    case "slo": return <SloList />;
  }
}
