"use client";

import Link from "next/link";
import { useParams } from "next/navigation";
import { useApi, type SpanRow } from "@/lib/api";
import { fmtMs } from "@/lib/format";
import { Card, ErrorNote, Loading } from "@/components/ui";

const palette = ["#5b8cff", "#2dd4a8", "#ffb547", "#c084fc", "#f472b6", "#38bdf8"];

export default function TracePage() {
  const { id } = useParams<{ id: string }>();
  const { data, error, loading } = useApi<SpanRow[]>(`/api/v1/traces/${id}`);
  if (loading) return <Loading />;
  if (!data) return <main className="p-6"><ErrorNote message={error ?? "Not found"} /></main>;

  const t0 = Math.min(...data.map((s) => +new Date(s.start)));
  const end = Math.max(...data.map((s) => +new Date(s.start) + s.durationMs));
  const total = Math.max(end - t0, 0.001);
  const services = [...new Set(data.map((s) => s.service))];

  // Depth-first order so children render under their parents.
  const byParent = new Map<string, SpanRow[]>();
  data.forEach((s) => byParent.set(s.parentSpanId, [...(byParent.get(s.parentSpanId) ?? []), s]));
  const ids = new Set(data.map((s) => s.spanId));
  const ordered: { s: SpanRow; depth: number }[] = [];
  const walk = (s: SpanRow, depth: number) => { ordered.push({ s, depth }); (byParent.get(s.spanId) ?? []).forEach((c) => walk(c, depth + 1)); };
  data.filter((s) => !s.parentSpanId || !ids.has(s.parentSpanId)).forEach((r) => walk(r, 0));

  return (
    <main className="mx-auto max-w-5xl px-4 py-8 sm:px-6">
      <Link href="/dashboard/traces" className="text-sm text-muted hover:text-fg">← Traces</Link>
      <h1 className="mt-3 text-xl font-semibold tracking-tight">Trace <span className="font-mono text-base text-muted">{id}</span></h1>
      <p className="mt-1 text-sm text-muted">{data.length} spans · {services.length} services · {fmtMs(total)}</p>
      <Card className="mt-6 overflow-x-auto p-4">
        <div className="min-w-[640px] space-y-1.5">
          {ordered.map(({ s, depth }) => {
            const left = ((+new Date(s.start) - t0) / total) * 100;
            const width = Math.max((s.durationMs / total) * 100, 0.4);
            const bad = s.statusCode === 2;
            return (
              <div key={s.spanId} className="grid grid-cols-[220px_1fr] items-center gap-3 text-xs">
                <div className="truncate" style={{ paddingLeft: depth * 12 }} title={`${s.service} · ${s.name}`}>
                  <span className="text-muted">{s.service}</span> {s.name}
                </div>
                <div className="relative h-5 rounded bg-surface-2">
                  <div
                    className="absolute top-0 flex h-5 items-center overflow-hidden rounded px-1.5 font-mono text-[10px] text-bg"
                    style={{ left: `${left}%`, width: `${width}%`, background: bad ? "var(--danger)" : palette[services.indexOf(s.service) % palette.length] }}
                    title={bad ? `${s.statusMessage}` : undefined}
                  >{width > 8 ? fmtMs(s.durationMs) : ""}</div>
                </div>
              </div>
            );
          })}
        </div>
      </Card>
      {data.some((s) => s.statusCode === 2) && (
        <Card className="mt-6 p-4 text-sm">
          <h2 className="font-medium">Errors</h2>
          <ul className="mt-2 space-y-1 text-muted">
            {data.filter((s) => s.statusCode === 2).map((s) => <li key={s.spanId}><span className="text-danger">{s.service}</span> {s.name}: {s.statusMessage || "error"}</li>)}
          </ul>
        </Card>
      )}
    </main>
  );
}
