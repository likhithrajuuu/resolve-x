"use client";

import Link from "next/link";
import { useSearchParams } from "next/navigation";
import { useState } from "react";
import { useApi, type ServiceRow, type TraceRow } from "@/lib/api";
import { fmtMs, timeAgo } from "@/lib/format";
import { Card, Empty, ErrorNote, Loading, PageHeader, WindowSelect } from "@/components/ui";

export default function TracesPage() {
  const [win, setWin] = useState("1h");
  const params = useSearchParams();
  const [service, setService] = useState(params.get("service") ?? "");
  const [errorsOnly, setErrorsOnly] = useState(params.get("status") === "error");

  const services = useApi<ServiceRow[]>(`/api/v1/services?window=${win}`);
  const qs = new URLSearchParams({ window: win, limit: "50" });
  if (service) qs.set("service", service);
  if (errorsOnly) qs.set("status", "error");
  const { data, error, loading } = useApi<TraceRow[]>(`/api/v1/traces?${qs}`, 10000);

  return (
    <main className="mx-auto max-w-5xl px-4 py-8 sm:px-6">
      <PageHeader title="Traces">
        <select aria-label="Service" value={service} onChange={(e) => setService(e.target.value)} className="rounded-md border border-line bg-surface px-2.5 py-1.5 text-sm">
          <option value="">All services</option>
          {(services.data ?? []).map((s) => <option key={s.name}>{s.name}</option>)}
        </select>
        <label className="flex items-center gap-2 text-sm text-muted"><input type="checkbox" checked={errorsOnly} onChange={(e) => setErrorsOnly(e.target.checked)} /> Errors only</label>
        <WindowSelect value={win} onChange={setWin} />
      </PageHeader>
      <Card className="mt-6 overflow-x-auto">
        {error && <div className="p-4"><ErrorNote message={error} /></div>}
        {loading && !data ? <Loading /> : !data?.length ? <Empty>No traces match.</Empty> : (
          <table className="w-full min-w-[640px] text-sm">
            <thead className="border-b border-line text-left text-xs text-muted"><tr>{["Root span", "Service", "Duration", "Spans", "Services", "Status", "Started"].map((h) => <th key={h} className="px-4 py-3 font-medium">{h}</th>)}</tr></thead>
            <tbody className="divide-y divide-line">
              {data.map((t) => (
                <tr key={t.traceId} className="hover:bg-surface-2">
                  <td className="px-4 py-2.5"><Link className="text-accent" href={`/dashboard/traces/${t.traceId}`}>{t.name}</Link></td>
                  <td className="px-4 py-2.5">{t.service}</td>
                  <td className="px-4 py-2.5">{fmtMs(t.durationMs)}</td>
                  <td className="px-4 py-2.5">{t.spans}</td>
                  <td className="px-4 py-2.5">{t.services}</td>
                  <td className="px-4 py-2.5">{t.errors ? <span className="text-danger">{t.errors} error{t.errors > 1 ? "s" : ""}</span> : <span className="text-accent-2">ok</span>}</td>
                  <td className="px-4 py-2.5 text-muted">{timeAgo(t.start)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </Card>
    </main>
  );
}
