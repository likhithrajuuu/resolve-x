"use client";

import Link from "next/link";
import { useParams } from "next/navigation";
import { useState } from "react";
import { useApi, type Edge, type Incident, type Point } from "@/lib/api";
import { fmtMs, fmtPct, timeAgo } from "@/lib/format";
import { ServiceCharts } from "@/components/Charts";
import { Card, ErrorNote, Loading, PageHeader, SeverityBadge, StatusLabel, WindowSelect } from "@/components/ui";

export default function ServicePage() {
  const { name: raw } = useParams<{ name: string }>();
  const name = decodeURIComponent(raw);
  const [win, setWin] = useState("1h");
  const series = useApi<Point[]>(`/api/v1/services/${encodeURIComponent(name)}/timeseries?window=${win}`, 10000);
  const edges = useApi<Edge[]>(`/api/v1/dependencies?window=${win}`, 15000);
  const incidents = useApi<Incident[]>("/api/v1/incidents", 15000);

  const up = (edges.data ?? []).filter((e) => e.target === name);
  const down = (edges.data ?? []).filter((e) => e.source === name);
  const related = (incidents.data ?? []).filter((i) => i.service === name).slice(0, 5);

  return (
    <main className="mx-auto max-w-5xl px-4 py-8 sm:px-6">
      <Link href="/dashboard/services" className="text-sm text-muted hover:text-fg">← Services</Link>
      <div className="mt-3"><PageHeader title={name}><WindowSelect value={win} onChange={setWin} /></PageHeader></div>
      <Card className="mt-6 p-5">
        {series.error ? <ErrorNote message={series.error} /> : series.loading && !series.data ? <Loading /> : <ServiceCharts points={series.data ?? []} />}
      </Card>
      <div className="mt-6 grid gap-6 md:grid-cols-2">
        <EdgeList title="Called by" edges={up} pick="source" />
        <EdgeList title="Depends on" edges={down} pick="target" />
      </div>
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
