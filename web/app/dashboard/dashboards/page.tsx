"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useState, type FormEvent } from "react";
import { api, useApi, type DashboardDef } from "@/lib/api";
import { timeAgo } from "@/lib/format";
import { btn, Card, Empty, ErrorNote, input, Loading, PageHeader } from "@/components/ui";

const TEMPLATES: Record<string, { label: string; widgets: DashboardDef["widgets"] }> = {
  blank: { label: "Blank", widgets: [] },
  overview: {
    label: "Operations overview",
    widgets: [
      { id: "w1", type: "incidents", title: "Active incidents", span: 1, config: {} },
      { id: "w2", type: "slo", title: "SLO status", span: 1, config: {} },
      { id: "w3", type: "logs", title: "Log volume", span: 2, config: {} },
    ],
  },
};

export default function DashboardsPage() {
  const router = useRouter();
  const { data, error, loading } = useApi<DashboardDef[]>("/api/v1/dashboards");
  const [err, setErr] = useState<string | null>(null);

  async function create(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const f = new FormData(e.currentTarget);
    try {
      const d = await api<DashboardDef>("/api/v1/dashboards", { method: "POST", body: JSON.stringify({ name: f.get("name"), widgets: TEMPLATES[String(f.get("template"))].widgets }) });
      router.push(`/dashboard/dashboards/${d.id}`);
    } catch (e2) { setErr(e2 instanceof Error ? e2.message : "Could not create"); }
  }

  return (
    <main className="mx-auto max-w-4xl px-4 py-8 sm:px-6">
      <PageHeader title="Dashboards" />
      <Card className="mt-6 p-4">
        <form onSubmit={create} className="flex flex-wrap items-end gap-3">
          <label className="text-sm text-muted">Name<input name="name" required maxLength={120} placeholder="Checkout health" className={`${input} mt-1 w-64`} /></label>
          <label className="text-sm text-muted">Start from<select name="template" className={`${input} mt-1 w-52`}>{Object.entries(TEMPLATES).map(([k, t]) => <option key={k} value={k}>{t.label}</option>)}</select></label>
          <button className={btn}>Create dashboard</button>
        </form>
        {err && <div className="mt-3"><ErrorNote message={err} /></div>}
      </Card>
      <Card className="mt-6 divide-y divide-line">
        {error && <div className="p-4"><ErrorNote message={error} /></div>}
        {loading && !data ? <Loading /> : !data?.length ? <Empty>No dashboards yet. Create one above, or add widgets from the Metrics explorer.</Empty> : data.map((d) => (
          <Link key={d.id} href={`/dashboard/dashboards/${d.id}`} className="flex items-center justify-between p-4 hover:bg-surface-2">
            <span className="font-medium">{d.name}</span>
            <span className="text-xs text-muted">{d.widgets.length} widgets · updated {timeAgo(d.updatedAt)}</span>
          </Link>
        ))}
      </Card>
      <p className="mt-4 text-xs text-muted">Dashboards are shared with everyone in your organization.</p>
    </main>
  );
}
