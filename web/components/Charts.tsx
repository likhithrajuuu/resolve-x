"use client";

import type { Point } from "@/lib/api";

/** Minimal dependency-free area/line chart. */
export function Series({ points, color = "var(--accent)", label, format }: {
  points: { t: string; v: number }[]; color?: string; label: string; format: (v: number) => string;
}) {
  const W = 400, H = 90;
  if (points.length < 2) return <div className="grid h-[90px] place-items-center text-xs text-muted">Not enough data</div>;
  const max = Math.max(...points.map((p) => p.v), 1e-9);
  const x = (i: number) => (i / (points.length - 1)) * W;
  const y = (v: number) => H - 4 - (v / max) * (H - 12);
  const line = points.map((p, i) => `${i ? "L" : "M"}${x(i).toFixed(1)},${y(p.v).toFixed(1)}`).join(" ");
  const last = points[points.length - 1].v;
  return (
    <figure>
      <figcaption className="mb-1 flex items-baseline justify-between text-xs text-muted">
        <span>{label}</span>
        <span className="font-mono text-fg">{format(last)} <span className="text-muted">· max {format(max)}</span></span>
      </figcaption>
      <svg viewBox={`0 0 ${W} ${H}`} className="h-[90px] w-full" role="img" aria-label={`${label} over time`} preserveAspectRatio="none">
        <path d={`${line} L${W},${H} L0,${H} Z`} fill={color} opacity="0.12" />
        <path d={line} fill="none" stroke={color} strokeWidth="1.5" vectorEffect="non-scaling-stroke" />
      </svg>
    </figure>
  );
}

export function ServiceCharts({ points }: { points: Point[] }) {
  return (
    <div className="grid gap-6 sm:grid-cols-3">
      <Series label="Requests / bucket" points={points.map((p) => ({ t: p.t, v: p.count }))} format={(v) => Math.round(v).toString()} />
      <Series label="Error rate" color="var(--danger)" points={points.map((p) => ({ t: p.t, v: p.count ? p.errors / p.count : 0 }))} format={(v) => `${(v * 100).toFixed(1)}%`} />
      <Series label="p95 latency" color="var(--warn)" points={points.map((p) => ({ t: p.t, v: p.p95Ms }))} format={(v) => `${v.toFixed(0)} ms`} />
    </div>
  );
}
