"use client";

import Link from "next/link";
import { useMemo, useState } from "react";
import { useApi, type Edge } from "@/lib/api";
import { fmtMs, fmtPct } from "@/lib/format";
import { Card, Empty, ErrorNote, Loading, PageHeader, WindowSelect } from "@/components/ui";

/** Layered layout: depth = longest path from any root; x spreads each layer. */
function layout(edges: Edge[]) {
  const nodes = [...new Set(edges.flatMap((e) => [e.source, e.target]))];
  const depth = new Map(nodes.map((n) => [n, 0]));
  for (let i = 0; i < nodes.length; i++) {
    for (const e of edges) {
      if (depth.get(e.target)! < depth.get(e.source)! + 1 && depth.get(e.source)! < nodes.length) depth.set(e.target, depth.get(e.source)! + 1);
    }
  }
  const layers: string[][] = [];
  for (const n of nodes) (layers[depth.get(n)!] ??= []).push(n);
  const pos = new Map<string, { x: number; y: number }>();
  const W = 760, rowH = 110;
  layers.forEach((layer, d) => layer.sort().forEach((n, i) => pos.set(n, { x: ((i + 1) * W) / (layer.length + 1), y: 50 + d * rowH })));
  return { pos, height: 100 + (layers.length - 1) * rowH, W };
}

export default function DependenciesPage() {
  const [win, setWin] = useState("1h");
  const { data, error, loading } = useApi<Edge[]>(`/api/v1/dependencies?window=${win}`, 15000);
  const g = useMemo(() => layout(data ?? []), [data]);
  const external = new Set((data ?? []).filter((e) => e.kind === "external").map((e) => e.target));

  return (
    <main className="mx-auto max-w-5xl px-4 py-8 sm:px-6">
      <PageHeader title="Dependencies"><WindowSelect value={win} onChange={setWin} /></PageHeader>
      <p className="mt-2 text-sm text-muted">Built from observed calls between services. Red edges have an error rate of 5% or more.</p>
      <Card className="mt-6 overflow-x-auto p-2">
        {error ? <div className="p-3"><ErrorNote message={error} /></div> : loading && !data ? <Loading /> : !data?.length ? (
          <Empty>No dependencies observed yet. They appear once traces with parent/child spans across services arrive.</Empty>
        ) : (
          <svg viewBox={`0 0 ${g.W} ${g.height}`} className="mx-auto min-w-[560px]" role="img" aria-label="Service dependency graph">
            <defs>
              <marker id="arrow" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="7" markerHeight="7" orient="auto-start-reverse">
                <path d="M0,0 L10,5 L0,10 z" fill="var(--muted)" />
              </marker>
            </defs>
            {data.map((e) => {
              const a = g.pos.get(e.source)!, b = g.pos.get(e.target)!;
              const bad = e.calls > 0 && e.errors / e.calls >= 0.05;
              return (
                <g key={e.source + e.target}>
                  <line x1={a.x} y1={a.y + 18} x2={b.x} y2={b.y - 20} stroke={bad ? "var(--danger)" : "var(--line)"} strokeWidth={bad ? 2 : 1.5} markerEnd="url(#arrow)" />
                  <text x={(a.x + b.x) / 2 + 6} y={(a.y + b.y) / 2} fontSize="10" fill="var(--muted)">{fmtMs(e.p95Ms)}</text>
                </g>
              );
            })}
            {[...g.pos].map(([n, p]) => (
              <Link key={n} href={external.has(n) ? "#" : `/dashboard/services/${encodeURIComponent(n)}`}>
                <g>
                  <rect x={p.x - 62} y={p.y - 18} width="124" height="36" rx="8" fill="var(--surface-2)" stroke={external.has(n) ? "var(--line)" : "var(--accent)"} strokeDasharray={external.has(n) ? "4 3" : undefined} />
                  <text x={p.x} y={p.y + 4} textAnchor="middle" fontSize="12" fill="var(--fg)">{n.length > 17 ? n.slice(0, 16) + "…" : n}</text>
                </g>
              </Link>
            ))}
          </svg>
        )}
      </Card>
      {!!data?.length && (
        <Card className="mt-6 overflow-x-auto">
          <table className="w-full min-w-[520px] text-sm">
            <thead className="border-b border-line text-left text-xs text-muted"><tr>{["From", "To", "Calls", "Errors", "p95"].map((h) => <th key={h} className="px-4 py-3 font-medium">{h}</th>)}</tr></thead>
            <tbody className="divide-y divide-line">
              {data.map((e) => (
                <tr key={e.source + e.target}><td className="px-4 py-2.5">{e.source}</td><td className="px-4 py-2.5">{e.target}</td><td className="px-4 py-2.5">{e.calls}</td>
                  <td className={`px-4 py-2.5 ${e.errors / e.calls >= 0.05 ? "text-danger" : ""}`}>{fmtPct(e.errors / e.calls)}</td><td className="px-4 py-2.5">{fmtMs(e.p95Ms)}</td></tr>
              ))}
            </tbody>
          </table>
        </Card>
      )}
    </main>
  );
}
