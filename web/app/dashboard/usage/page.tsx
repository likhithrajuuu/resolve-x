"use client";

import { useState } from "react";
import { useApi, type Usage } from "@/lib/api";
import { fmtNum } from "@/lib/format";
import { StackedBars } from "@/components/charts";
import { Card, ErrorNote, Loading, PageHeader } from "@/components/ui";

export default function UsagePage() {
  const [days, setDays] = useState(30);
  const { data, error, loading } = useApi<Usage>(`/api/v1/usage?limit=${days}`, 60000);
  const tiles = data ? [["Spans", data.total.spans], ["Log lines", data.total.logs], ["Metric points", data.total.metricPoints], ["Services", data.services]] as const : [];
  return (
    <main className="mx-auto max-w-4xl px-4 py-8 sm:px-6">
      <PageHeader title="Usage">
        <select aria-label="Period" value={days} onChange={(e) => setDays(+e.target.value)} className="rounded-md border border-line bg-surface px-2.5 py-1.5 text-sm">
          {[7, 14, 30].map((d) => <option key={d} value={d}>Last {d} days</option>)}
        </select>
      </PageHeader>
      <p className="mt-2 text-sm text-muted">Telemetry stored for your organization. Telemetry older than its retention (logs 14 days, traces and metrics 30 days) is deleted automatically.</p>
      {error && <div className="mt-4"><ErrorNote message={error} /></div>}
      {loading && !data ? <Loading /> : data && (<>
        <dl className="mt-6 grid grid-cols-2 gap-3 lg:grid-cols-4">
          {tiles.map(([l, v]) => <Card key={l} className="p-4"><dt className="text-xs text-muted">{l}</dt><dd className="mt-1 text-xl font-semibold">{fmtNum(v)}</dd></Card>)}
        </dl>
        <Card className="mt-6 p-4">
          <h2 className="mb-3 text-sm font-medium">Per day</h2>
          <StackedBars height={160} data={data.days.map((d) => ({ t: +new Date(d.day), parts: [
            { value: d.spans, color: "var(--accent)", label: "spans" }, { value: d.logs, color: "var(--accent-2)", label: "logs" }, { value: d.metricPoints, color: "var(--warn)", label: "metric points" }] }))} />
          <p className="mt-2 flex gap-3 text-xs text-muted"><span style={{ color: "var(--accent)" }}>■ spans</span><span style={{ color: "var(--accent-2)" }}>■ logs</span><span style={{ color: "var(--warn)" }}>■ metric points</span></p>
        </Card>
      </>)}
    </main>
  );
}
