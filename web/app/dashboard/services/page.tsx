"use client";

import Link from "next/link";
import { useState } from "react";
import { useApi, type ServiceRow } from "@/lib/api";
import { fmtMs, fmtNum, fmtPct, timeAgo } from "@/lib/format";
import { Card, Empty, ErrorNote, Loading, PageHeader, WindowSelect } from "@/components/ui";

export default function ServicesPage() {
  const [win, setWin] = useState("1h");
  const { data, error, loading } = useApi<ServiceRow[]>(`/api/v1/services?window=${win}`, 10000);
  return (
    <main className="mx-auto max-w-5xl px-4 py-8 sm:px-6">
      <PageHeader title="Services"><WindowSelect value={win} onChange={setWin} /></PageHeader>
      <p className="mt-2 text-sm text-muted">Discovered automatically from telemetry. Nothing to register.</p>
      <Card className="mt-6 overflow-x-auto">
        {error && <div className="p-4"><ErrorNote message={error} /></div>}
        {loading && !data ? <Loading /> : !data?.length ? (
          <Empty>No services yet. Send OpenTelemetry traces to Resolve-X and they will appear here.</Empty>
        ) : (
          <table className="w-full min-w-[640px] text-sm">
            <thead className="border-b border-line text-left text-xs text-muted">
              <tr>{["Service", "Requests", "Error rate", "p50", "p95", "p99", "Last seen"].map((h) => <th key={h} className="px-4 py-3 font-medium">{h}</th>)}</tr>
            </thead>
            <tbody className="divide-y divide-line">
              {data.map((s) => (
                <tr key={s.name} className="hover:bg-surface-2">
                  <td className="px-4 py-3 font-medium"><Link className="text-accent" href={`/dashboard/services/${encodeURIComponent(s.name)}`}>{s.name}</Link></td>
                  <td className="px-4 py-3">{fmtNum(s.spans)}</td>
                  <td className={`px-4 py-3 ${s.errorRate >= 0.05 ? "text-danger" : ""}`}>{fmtPct(s.errorRate)}</td>
                  <td className="px-4 py-3">{fmtMs(s.p50Ms)}</td>
                  <td className="px-4 py-3">{fmtMs(s.p95Ms)}</td>
                  <td className="px-4 py-3">{fmtMs(s.p99Ms)}</td>
                  <td className="px-4 py-3 text-muted">{timeAgo(s.lastSeen)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </Card>
    </main>
  );
}
