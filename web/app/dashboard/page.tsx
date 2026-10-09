"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { api, useApi, type Deployment, type Incident, type IncidentDetail, type Point, type ServiceRow } from "@/lib/api";
import { fmtMs, fmtNum, fmtPct, timeAgo } from "@/lib/format";
import { Legend, LineChart, type Line } from "@/components/charts";
import { SloList } from "@/components/widgets";
import { btn, Card, ErrorNote, Loading, PageHeader, SeverityBadge, StatusLabel, WindowSelect } from "@/components/ui";

const health = (s: ServiceRow) => (s.errorRate >= 0.05 ? "bad" : s.errorRate >= 0.01 ? "warn" : "ok") as "bad" | "warn" | "ok";
const tone = { ok: "border-accent-2/30 bg-accent-2/5", warn: "border-warn/40 bg-warn/5", bad: "border-danger/50 bg-danger/10" } as const;
const dot = { ok: "bg-accent-2", warn: "bg-warn", bad: "bg-danger" } as const;

export default function Overview() {
  const [win, setWin] = useState("1h");
  const services = useApi<ServiceRow[]>(`/api/v1/services?window=${win}`, 10000);
  const incidents = useApi<Incident[]>("/api/v1/incidents", 8000);
  const deploys = useApi<Deployment[]>("/api/v1/deployments", 30000);

  const active = (incidents.data ?? []).filter((i) => i.status !== "resolved");
  const svc = services.data ?? [];
  const unhealthy = svc.filter((s) => health(s) !== "ok").length;
  const noData = services.data !== null && svc.length === 0 && incidents.data !== null && (incidents.data?.length ?? 0) === 0;

  return (
    <main className="mx-auto max-w-6xl px-4 py-8 sm:px-6">
      <PageHeader title="Overview"><WindowSelect value={win} onChange={setWin} /></PageHeader>

      {noData ? (
        <Card className="mt-6 border-accent/40 p-6">
          <h2 className="text-lg font-medium">Connect your first application</h2>
          <p className="mt-1 text-sm text-muted">No telemetry yet. Install an SDK, add your API key, and services, incidents and SLOs appear here.</p>
          <Link href="/dashboard/settings" className={`${btn} mt-4 inline-block`}>Get your API key and SDK snippet</Link>
        </Card>
      ) : (
        <>
          <div role="status" className={`mt-6 flex flex-wrap items-center gap-x-6 gap-y-1 rounded-xl border px-5 py-4 ${active.length ? "border-danger/50 bg-danger/10" : unhealthy ? "border-warn/40 bg-warn/5" : "border-accent-2/30 bg-accent-2/5"}`}>
            <p className="text-lg font-medium">
              {active.length ? `${active.length} active incident${active.length > 1 ? "s" : ""}` : unhealthy ? `${unhealthy} service${unhealthy > 1 ? "s" : ""} degraded` : services.data ? "All systems healthy" : "Checking…"}
            </p>
            {services.data && <p className="text-sm text-muted">{svc.length} services · {fmtNum(svc.reduce((a, s) => a + s.spans, 0))} requests · {fmtPct(svc.reduce((a, s) => a + s.errors, 0) / Math.max(svc.reduce((a, s) => a + s.spans, 0), 1))} errors</p>}
          </div>

          {active.length > 0 && (
            <section className="mt-8" aria-labelledby="attn">
              <h2 id="attn" className="text-sm font-medium text-muted">Needs attention</h2>
              <div className="mt-3 grid gap-4 lg:grid-cols-2">{active.slice(0, 4).map((i) => <IncidentCard key={i.id} incident={i} />)}</div>
            </section>
          )}

          <section className="mt-8" aria-labelledby="svc">
            <div className="flex items-baseline justify-between"><h2 id="svc" className="text-sm font-medium text-muted">Service health</h2><Link href="/dashboard/dependencies" className="text-xs text-accent">View map →</Link></div>
            {services.error && <div className="mt-3"><ErrorNote message={services.error} /></div>}
            {services.loading && !services.data ? <Loading /> : (
              <div className="mt-3 grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
                {svc.map((s) => (
                  <Link key={s.name} href={`/dashboard/services/${encodeURIComponent(s.name)}`} className={`rounded-xl border p-4 transition-colors hover:bg-surface-2 ${tone[health(s)]}`}>
                    <p className="flex items-center gap-2 font-medium"><span className={`h-2 w-2 rounded-full ${dot[health(s)]}`} />{s.name}</p>
                    <p className="mt-2 grid grid-cols-3 gap-1 text-xs text-muted">
                      <span><b className="block text-sm text-fg">{fmtNum(s.spans)}</b>req</span>
                      <span><b className={`block text-sm ${s.errorRate >= 0.05 ? "text-danger" : "text-fg"}`}>{fmtPct(s.errorRate)}</b>err</span>
                      <span><b className="block text-sm text-fg">{fmtMs(s.p95Ms)}</b>p95</span>
                    </p>
                  </Link>
                ))}
              </div>
            )}
          </section>

          <div className="mt-8 grid gap-6 lg:grid-cols-3">
            <section className="lg:col-span-2" aria-labelledby="traffic">
              <h2 id="traffic" className="text-sm font-medium text-muted">Traffic by service</h2>
              <Card className="mt-3 p-4"><Fleet services={svc.slice(0, 6)} win={win} deploys={deploys.data ?? []} /></Card>
            </section>
            <section aria-labelledby="slo">
              <div className="flex items-baseline justify-between"><h2 id="slo" className="text-sm font-medium text-muted">SLOs</h2><Link href="/dashboard/slos" className="text-xs text-accent">Manage →</Link></div>
              <Card className="mt-3 p-4"><SloList /></Card>
              <h2 className="mt-6 text-sm font-medium text-muted">Recent deployments</h2>
              <Card className="mt-3 divide-y divide-line">
                {(deploys.data ?? []).slice(0, 5).map((d) => (
                  <div key={d.id} className="flex items-center justify-between gap-2 px-4 py-2.5 text-sm"><span className="truncate">{d.service} <span className="font-mono text-xs text-accent">{d.version}</span></span><span className="shrink-0 text-xs text-muted">{timeAgo(d.at)}</span></div>
                ))}
                {!deploys.data?.length && <p className="p-4 text-sm text-muted">None recorded. Send them with the SDK&apos;s <code className="font-mono text-xs">markDeployment()</code>.</p>}
              </Card>
            </section>
          </div>
        </>
      )}
    </main>
  );
}

function IncidentCard({ incident }: { incident: Incident }) {
  const { data } = useApi<IncidentDetail>(`/api/v1/incidents/${incident.id}`, 8000);
  const a = data?.analyses[0];
  return (
    <Link href={`/dashboard/incidents/${incident.id}`} className="block rounded-xl border border-line bg-surface p-5 transition-colors hover:bg-surface-2">
      <div className="flex items-center gap-3"><SeverityBadge s={incident.severity} /><StatusLabel s={incident.status} /><span className="ml-auto text-xs text-muted">{timeAgo(incident.createdAt)}</span></div>
      <p className="mt-3 font-medium">{incident.title}</p>
      {a ? (
        <>
          <p className="mt-2 text-xs uppercase tracking-wider text-accent-2">Probable cause · {a.confidence}%</p>
          <p className="mt-1 text-sm">{a.rootCause}</p>
          {a.recommendation.description && <p className="mt-2 text-sm text-muted">→ {a.recommendation.description}</p>}
          {a.approval === "pending" && <p className="mt-3 text-xs text-warn">Awaiting your approval</p>}
        </>
      ) : <p className="mt-2 text-sm text-muted">Analysing evidence…</p>}
    </Link>
  );
}

/** Request volume per service, with deployment markers. */
function Fleet({ services, win, deploys }: { services: ServiceRow[]; win: string; deploys: Deployment[] }) {
  const [lines, setLines] = useState<Line[] | null>(null);
  const key = services.map((s) => s.name).join("|") + win;
  useEffect(() => {
    if (!services.length) return;
    let live = true;
    const load = () => Promise.all(services.map((s) => api<Point[]>(`/api/v1/services/${encodeURIComponent(s.name)}/timeseries?window=${win}`).then((pts) => ({ name: s.name, points: pts.map((p) => ({ t: +new Date(p.t), v: p.count })) })).catch(() => null)))
      .then((r) => { if (live) setLines(r.filter((x): x is Line => !!x)); });
    void load();
    const id = setInterval(load, 15000);
    return () => { live = false; clearInterval(id); };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [key]);
  if (!lines) return <Loading />;
  return <><LineChart lines={lines} height={200} format={(v) => fmtNum(Math.round(v))} markers={deploys.map((d) => ({ t: +new Date(d.at), label: `${d.service} ${d.version}` }))} /><Legend lines={lines} /></>;
}
