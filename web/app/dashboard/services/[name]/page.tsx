"use client";

import Link from "next/link";
import { useParams } from "next/navigation";
import { useState } from "react";
import { useApi, type Deployment, type Edge, type Incident, type OpRow, type Point } from "@/lib/api";
import { fmtMs, fmtPct, timeAgo } from "@/lib/format";
import { LineChart, type Line } from "@/components/charts";
import { Card, ErrorNote, Loading, PageHeader, SeverityBadge, StatusLabel, WindowSelect } from "@/components/ui";

export default function ServicePage() {
  const { name: raw } = useParams<{ name: string }>();
  const name = decodeURIComponent(raw);
  const [win, setWin] = useState("1h");
  const series = useApi<Point[]>(`/api/v1/services/${encodeURIComponent(name)}/timeseries?window=${win}`, 10000);
  const edges = useApi<Edge[]>(`/api/v1/dependencies?window=${win}`, 15000);
  const incidents = useApi<Incident[]>("/api/v1/incidents", 15000);
  const ops = useApi<OpRow[]>(`/api/v1/services/${encodeURIComponent(name)}/operations?window=${win}`, 15000);
  const deploys = useApi<Deployment[]>("/api/v1/deployments", 30000);
  const pts = series.data ?? [];
  const markers = (deploys.data ?? []).filter((d) => d.service === name).map((d) => ({ t: +new Date(d.at), label: d.version }));
  const mk = (f: (p: Point) => number, color?: string): Line[] => [{ name: "", color, points: pts.map((p) => ({ t: +new Date(p.t), v: f(p) })) }];

  const up = (edges.data ?? []).filter((e) => e.target === name);
  const down = (edges.data ?? []).filter((e) => e.source === name);
  const related = (incidents.data ?? []).filter((i) => i.service === name).slice(0, 5);

  return (
    <main className="mx-auto max-w-5xl px-4 py-8 sm:px-6">
      <Link href="/dashboard/services" className="text-sm text-muted hover:text-fg">← Services</Link>
      <div className="mt-3"><PageHeader title={name}><WindowSelect value={win} onChange={setWin} /></PageHeader></div>
      <Card className="mt-6 p-5">
        {series.error ? <ErrorNote message={series.error} /> : series.loading && !series.data ? <Loading /> : <div className="grid gap-5 sm:grid-cols-3">
          <div><p className="mb-1 text-xs text-muted">Requests</p><LineChart lines={mk((p) => p.count)} height={130} area markers={markers} format={(v) => String(Math.round(v))} /></div>
          <div><p className="mb-1 text-xs text-muted">Error rate</p><LineChart lines={mk((p) => (p.count ? p.errors / p.count : 0), "var(--danger)")} height={130} area markers={markers} format={(v) => `${(v * 100).toFixed(1)}%`} /></div>
          <div><p className="mb-1 text-xs text-muted">p95 latency</p><LineChart lines={mk((p) => p.p95Ms, "var(--warn)")} height={130} area markers={markers} format={fmtMs} /></div>
        </div>}
      </Card>
      <div className="mt-6 grid gap-6 md:grid-cols-2">
        <EdgeList title="Called by" edges={up} pick="source" />
        <EdgeList title="Depends on" edges={down} pick="target" />
      </div>
      <h2 className="mt-8 text-sm font-medium text-muted">Operations</h2>
      <Card className="mt-3 overflow-x-auto">
        {ops.loading && !ops.data ? <Loading /> : !ops.data?.length ? <p className="p-4 text-sm text-muted">No entry operations in this window.</p> : (
          <table className="w-full min-w-[560px] text-sm">
            <thead className="border-b border-line text-left text-xs text-muted"><tr>{["Operation", "Calls", "Error rate", "p50", "p95", "p99"].map((h) => <th key={h} className="px-4 py-2.5 font-medium">{h}</th>)}</tr></thead>
            <tbody className="divide-y divide-line">{ops.data.map((o) => (
              <tr key={o.name}><td className="px-4 py-2.5 font-mono text-xs">{o.name}</td><td className="px-4 py-2.5">{o.calls}</td>
                <td className={`px-4 py-2.5 ${o.errorRate >= 0.05 ? "text-danger" : ""}`}>{fmtPct(o.errorRate)}</td><td className="px-4 py-2.5">{fmtMs(o.p50Ms)}</td><td className="px-4 py-2.5">{fmtMs(o.p95Ms)}</td><td className="px-4 py-2.5">{fmtMs(o.p99Ms)}</td></tr>))}</tbody>
          </table>)}
      </Card>
      <h2 className="mt-8 text-sm font-medium text-muted">Recent incidents</h2>
      <Card className="mt-3 divide-y divide-line">
        {related.length === 0 ? <p className="p-4 text-sm text-muted">None.</p> : related.map((i) => (
          <Link key={i.id} href={`/dashboard/incidents/${i.id}`} className="flex items-center gap-3 p-4 text-sm hover:bg-surface-2">
            <SeverityBadge s={i.severity} /><span className="min-w-0 flex-1 truncate">{i.title}</span><StatusLabel s={i.status} /><span className="text-xs text-muted">{timeAgo(i.createdAt)}</span>
          </Link>
        ))}
      </Card>
      <p className="mt-6 text-sm"><Link className="text-accent" href={`/dashboard/traces?service=${encodeURIComponent(name)}`}>View traces</Link> · <Link className="text-accent" href={`/dashboard/logs?service=${encodeURIComponent(name)}`}>View logs</Link></p>
    </main>
  );
}

function EdgeList({ title, edges, pick }: { title: string; edges: Edge[]; pick: "source" | "target" }) {
  return (
    <section>
      <h2 className="text-sm font-medium text-muted">{title}</h2>
      <Card className="mt-3 divide-y divide-line">
        {edges.length === 0 ? <p className="p-4 text-sm text-muted">None observed.</p> : edges.map((e) => (
          <div key={e.source + e.target} className="flex items-center justify-between gap-3 p-4 text-sm">
            <span className="truncate">{e[pick]}{e.kind === "external" && <span className="ml-2 text-xs text-muted">external</span>}</span>
            <span className="shrink-0 text-xs text-muted">{e.calls} calls · <span className={e.errors / e.calls >= 0.05 ? "text-danger" : ""}>{fmtPct(e.errors / e.calls)}</span> err · p95 {fmtMs(e.p95Ms)}</span>
          </div>
        ))}
      </Card>
    </section>
  );
}
