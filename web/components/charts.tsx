"use client";

import { useMemo, useRef, useState } from "react";

export const SERIES_COLORS = ["#5b8cff", "#2dd4a8", "#ffb547", "#c084fc", "#f472b6", "#38bdf8", "#a3e635", "#fb923c"];

export type Line = { name: string; color?: string; points: { t: number; v: number }[] };

/**
 * Multi-series line chart with axes, gridlines, deployment markers and a hover
 * crosshair/tooltip. Dependency-free SVG so many widgets on one page stay fast.
 */
export function LineChart({ lines, height = 180, format = (v: number) => String(+v.toFixed(2)), area = false, markers = [] }: {
  lines: Line[]; height?: number; format?: (v: number) => string; area?: boolean; markers?: { t: number; label: string }[];
}) {
  const ref = useRef<SVGSVGElement>(null);
  const [hover, setHover] = useState<number | null>(null);
  const W = 640, padL = 44, padR = 8, padT = 8, padB = 20;

  const { tMin, tMax, vMax } = useMemo(() => {
    const all = lines.flatMap((l) => l.points);
    return {
      tMin: Math.min(...all.map((p) => p.t)),
      tMax: Math.max(...all.map((p) => p.t)),
      vMax: Math.max(...all.map((p) => p.v), 1e-9),
    };
  }, [lines]);

  if (!lines.some((l) => l.points.length > 1)) {
    return <div className="grid place-items-center text-xs text-muted" style={{ height }}>Not enough data in this window</div>;
  }
  const x = (t: number) => padL + ((t - tMin) / Math.max(tMax - tMin, 1)) * (W - padL - padR);
  const y = (v: number) => padT + (1 - v / vMax) * (height - padT - padB);
  const ticks = [0, 0.5, 1].map((f) => f * vMax);
  const color = (l: Line, i: number) => l.color ?? SERIES_COLORS[i % SERIES_COLORS.length];

  const onMove = (e: React.MouseEvent) => {
    const r = ref.current!.getBoundingClientRect();
    const px = ((e.clientX - r.left) / r.width) * W;
    setHover(tMin + ((px - padL) / (W - padL - padR)) * (tMax - tMin));
  };
  const nearest = (l: Line) => (hover === null || !l.points.length ? null : l.points.reduce((a, b) => (Math.abs(b.t - hover) < Math.abs(a.t - hover) ? b : a)));
  const hp = hover === null ? null : nearest(lines[0]);
  const fmtT = (t: number) => new Date(t).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit", hour12: false });

  return (
    <div className="relative">
      <svg ref={ref} viewBox={`0 0 ${W} ${height}`} className="w-full" style={{ height }} role="img" aria-label="Time series chart"
        onMouseMove={onMove} onMouseLeave={() => setHover(null)}>
        {ticks.map((tv) => (
          <g key={tv}>
            <line x1={padL} x2={W - padR} y1={y(tv)} y2={y(tv)} stroke="var(--line)" strokeDasharray="3 4" />
            <text x={padL - 6} y={y(tv) + 3} textAnchor="end" fontSize="10" fill="var(--muted)">{format(tv)}</text>
          </g>
        ))}
        {[0, 0.5, 1].map((f) => (
          <text key={f} x={x(tMin + f * (tMax - tMin))} y={height - 5} textAnchor={f === 0 ? "start" : f === 1 ? "end" : "middle"} fontSize="10" fill="var(--muted)">{fmtT(tMin + f * (tMax - tMin))}</text>
        ))}
        {markers.filter((m) => m.t >= tMin && m.t <= tMax).map((m) => (
          <g key={m.t + m.label}>
            <line x1={x(m.t)} x2={x(m.t)} y1={padT} y2={height - padB} stroke="var(--accent)" strokeDasharray="2 3" opacity="0.7" />
            <text x={x(m.t) + 3} y={padT + 9} fontSize="9" fill="var(--accent)">{m.label}</text>
          </g>
        ))}
        {lines.map((l, i) => {
          if (!l.points.length) return null;
          const d = l.points.map((p, j) => `${j ? "L" : "M"}${x(p.t).toFixed(1)},${y(p.v).toFixed(1)}`).join(" ");
          return (
            <g key={l.name}>
              {area && <path d={`${d} L${x(l.points[l.points.length - 1].t)},${y(0)} L${x(l.points[0].t)},${y(0)} Z`} fill={color(l, i)} opacity="0.12" />}
              <path d={d} fill="none" stroke={color(l, i)} strokeWidth="1.6" vectorEffect="non-scaling-stroke" />
            </g>
          );
        })}
        {hp && <line x1={x(hp.t)} x2={x(hp.t)} y1={padT} y2={height - padB} stroke="var(--fg)" opacity="0.35" />}
        {hp && lines.map((l, i) => { const p = nearest(l); return p ? <circle key={l.name} cx={x(p.t)} cy={y(p.v)} r="3" fill={color(l, i)} /> : null; })}
      </svg>
      {hp && (
        <div className="pointer-events-none absolute right-2 top-1 z-10 max-w-[60%] rounded-md border border-line bg-surface-2 px-2.5 py-1.5 text-xs shadow-lg">
          <p className="font-mono text-muted">{fmtT(hp.t)}</p>
          {lines.slice(0, 8).map((l, i) => {
            const p = nearest(l);
            return p ? (
              <p key={l.name} className="flex items-center gap-1.5">
                <span className="h-2 w-2 shrink-0 rounded-full" style={{ background: color(l, i) }} />
                <span className="truncate text-muted">{l.name || "value"}</span>
                <span className="ml-auto pl-2 font-mono">{format(p.v)}</span>
              </p>
            ) : null;
          })}
        </div>
      )}
    </div>
  );
}

export function Legend({ lines }: { lines: Line[] }) {
  if (lines.length < 2) return null;
  return (
    <ul className="mt-2 flex flex-wrap gap-x-4 gap-y-1 text-xs text-muted">
      {lines.map((l, i) => (
        <li key={l.name} className="flex items-center gap-1.5">
          <span className="h-2 w-2 rounded-full" style={{ background: l.color ?? SERIES_COLORS[i % SERIES_COLORS.length] }} />
          {l.name || "value"}
        </li>
      ))}
    </ul>
  );
}

/** Stacked bars, used for log volume by severity. */
export function StackedBars({ data, height = 120 }: { data: { t: number; parts: { value: number; color: string; label: string }[] }[]; height?: number }) {
  if (data.length < 2) return <div className="grid place-items-center text-xs text-muted" style={{ height }}>Not enough data</div>;
  const W = 640, gap = 1;
  const max = Math.max(...data.map((d) => d.parts.reduce((a, p) => a + p.value, 0)), 1);
  const bw = W / data.length;
  return (
    <svg viewBox={`0 0 ${W} ${height}`} className="w-full" style={{ height }} role="img" aria-label="Volume over time">
      {data.map((d, i) => {
        let yy = height;
        return (
          <g key={d.t}>
            {d.parts.map((p) => {
              const h = (p.value / max) * (height - 4);
              yy -= h;
              return h > 0 ? <rect key={p.label} x={i * bw + gap / 2} y={yy} width={Math.max(bw - gap, 1)} height={h} fill={p.color}><title>{`${new Date(d.t).toLocaleTimeString()} · ${p.label}: ${p.value}`}</title></rect> : null;
            })}
          </g>
        );
      })}
    </svg>
  );
}
