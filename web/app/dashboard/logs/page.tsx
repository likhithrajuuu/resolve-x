"use client";

import Link from "next/link";
import { useSearchParams } from "next/navigation";
import { useEffect, useState } from "react";
import { useApi, type LogPattern, type LogRow, type ServiceRow } from "@/lib/api";
import { LogVolumeChart } from "@/components/widgets";
import { fmtNum, fmtTime, timeAgo } from "@/lib/format";
import { Card, Empty, ErrorNote, input, Loading, PageHeader, WindowSelect } from "@/components/ui";

const tone = (n: number) => (n >= 17 ? "text-danger" : n >= 13 ? "text-warn" : "text-muted");

export default function LogsPage() {
  const [win, setWin] = useState("1h");
  const params = useSearchParams();
  const [service, setService] = useState(params.get("service") ?? "");
  const [minSeverity, setMinSeverity] = useState(params.get("minSeverity") ?? "");
  const [q, setQ] = useState("");
  const [debounced, setDebounced] = useState("");
  const [tab, setTab] = useState<"stream" | "patterns">("stream");
  useEffect(() => { const t = setTimeout(() => setDebounced(q), 300); return () => clearTimeout(t); }, [q]);

  const services = useApi<ServiceRow[]>(`/api/v1/services?window=${win}`);
  const qs = new URLSearchParams({ window: win, limit: "200" });
  if (service) qs.set("service", service);
  if (minSeverity) qs.set("minSeverity", minSeverity);
  if (debounced) qs.set("q", debounced);
  const { data, error, loading } = useApi<LogRow[]>(tab === "stream" ? `/api/v1/logs?${qs}` : null, 5000);
  const pq = new URLSearchParams({ window: win });
  if (service) pq.set("service", service);
  const patterns = useApi<LogPattern[]>(tab === "patterns" ? `/api/v1/logs/patterns?${pq}` : null, 15000);

  return (
    <main className="mx-auto max-w-5xl px-4 py-8 sm:px-6">
      <PageHeader title="Logs">
        <input aria-label="Search logs" placeholder="Search message…" value={q} onChange={(e) => setQ(e.target.value)} className={`${input} w-48`} />
        <select aria-label="Service" value={service} onChange={(e) => setService(e.target.value)} className="rounded-md border border-line bg-surface px-2.5 py-1.5 text-sm">
          <option value="">All services</option>
          {(services.data ?? []).map((s) => <option key={s.name}>{s.name}</option>)}
        </select>
        <select aria-label="Minimum severity" value={minSeverity} onChange={(e) => setMinSeverity(e.target.value)} className="rounded-md border border-line bg-surface px-2.5 py-1.5 text-sm">
          <option value="">All levels</option><option value="13">Warn+</option><option value="17">Error+</option>
        </select>
        <WindowSelect value={win} onChange={setWin} />
      </PageHeader>
      <Card className="mt-6 p-4"><LogVolumeChart service={service || undefined} win={win} /></Card>
      <div role="tablist" aria-label="Log views" className="mt-6 flex gap-1 border-b border-line text-sm">
        {([["stream", "Live stream"], ["patterns", "Patterns"]] as const).map(([k, l]) => (
          <button key={k} role="tab" aria-selected={tab === k} onClick={() => setTab(k)} className={`-mb-px border-b-2 px-4 py-2 ${tab === k ? "border-accent text-fg" : "border-transparent text-muted hover:text-fg"}`}>{l}</button>
        ))}
      </div>
      {tab === "patterns" ? (
        <Card className="mt-4 overflow-hidden">
          {patterns.error && <div className="p-4"><ErrorNote message={patterns.error} /></div>}
          {patterns.loading && !patterns.data ? <Loading /> : !patterns.data?.length ? <Empty>No logs in this window.</Empty> : (
            <ul className="divide-y divide-line">
              {patterns.data.map((p, i) => (
                <li key={i} className="flex gap-4 px-4 py-3">
                  <div className="w-16 shrink-0 text-right"><p className="text-lg font-semibold">{fmtNum(p.count)}</p><p className="text-[11px] text-muted">events</p></div>
                  <div className="min-w-0 flex-1">
                    <p className="break-words font-mono text-xs"><span className={tone(p.severityNumber)}>{p.severityNumber >= 17 ? "ERROR" : p.severityNumber >= 13 ? "WARN" : "INFO"}</span> {p.pattern}</p>
                    <p className="mt-1 text-xs text-muted"><span className="text-accent">{p.service}</span> · last {timeAgo(p.lastSeen)} · e.g. <span className="font-mono">{p.sample}</span></p>
                  </div>
                </li>
              ))}
            </ul>
          )}
        </Card>
      ) : (
      <Card className="mt-4 overflow-hidden">
        {error && <div className="p-4"><ErrorNote message={error} /></div>}
        {loading && !data ? <Loading /> : !data?.length ? <Empty>No logs match.</Empty> : (
          <ul className="divide-y divide-line font-mono text-xs">
            {data.map((l, i) => (
              <li key={i} className="flex flex-col gap-1 px-4 py-2 sm:flex-row sm:gap-3">
                <span className="shrink-0 text-muted">{fmtTime(l.ts)}</span>
                <span className={`w-12 shrink-0 ${tone(l.severityNumber)}`}>{l.severityText || "–"}</span>
                <span className="w-24 shrink-0 truncate text-accent">{l.service}</span>
                <span className="min-w-0 flex-1 break-words text-fg/90">{l.body}</span>
                {l.traceId && <Link className="shrink-0 text-muted hover:text-accent" href={`/dashboard/traces/${l.traceId}`}>trace</Link>}
              </li>
            ))}
          </ul>
        )}
      </Card>
      )}
    </main>
  );
}
