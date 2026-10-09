"use client";

import Link from "next/link";
import { useParams } from "next/navigation";
import { useState } from "react";
import { api, useApi, type Analysis, type IncidentDetail } from "@/lib/api";
import { fmtTime, timeAgo } from "@/lib/format";
import { btn, btnGhost, Card, ErrorNote, Loading, SeverityBadge, StatusLabel } from "@/components/ui";

const kindTone: Record<string, string> = { deploy: "text-accent", metric: "text-warn", dependency: "text-warn", trace: "text-warn", log: "text-danger" };

export default function IncidentPage() {
  const { id } = useParams<{ id: string }>();
  const { data: inc, error, loading, reload } = useApi<IncidentDetail>(`/api/v1/incidents/${id}`, 4000);
  const [actionError, setActionError] = useState<string | null>(null);

  async function act(path: string, init: RequestInit) {
    setActionError(null);
    try {
      await api(path, init);
      await reload();
    } catch (e) {
      setActionError(e instanceof Error ? e.message : "Action failed");
    }
  }

  if (loading && !inc) return <Loading />;
  if (!inc) return <main className="p-6"><ErrorNote message={error ?? "Not found"} /></main>;
  const latest: Analysis | undefined = inc.analyses[0];

  return (
    <main className="mx-auto max-w-5xl px-4 py-8 sm:px-6">
      <Link href="/dashboard" className="text-sm text-muted hover:text-fg">← Incidents</Link>
      <div className="mt-3 flex flex-wrap items-start justify-between gap-3">
        <div>
          <div className="flex items-center gap-3"><SeverityBadge s={inc.severity} /><StatusLabel s={inc.status} /></div>
          <h1 className="mt-2 text-2xl font-semibold tracking-tight">{inc.title}</h1>
          <p className="mt-1 text-sm text-muted">
            {inc.service ? <>Service <Link className="text-accent" href={`/dashboard/services/${encodeURIComponent(inc.service)}`}>{inc.service}</Link> · </> : null}
            opened {timeAgo(inc.createdAt)} via {inc.source}
          </p>
        </div>
        <div className="flex gap-2">
          <button className={btnGhost} onClick={() => act(`/api/v1/incidents/${inc.id}/analyze`, { method: "POST" })}>Re-analyze</button>
          {inc.status !== "resolved"
            ? <button className={btn} onClick={() => act(`/api/v1/incidents/${inc.id}`, { method: "PATCH", body: JSON.stringify({ status: "resolved" }) })}>Mark resolved</button>
            : <button className={btnGhost} onClick={() => act(`/api/v1/incidents/${inc.id}`, { method: "PATCH", body: JSON.stringify({ status: "investigating" }) })}>Reopen</button>}
        </div>
      </div>
      {inc.description && <p className="mt-4 text-sm text-fg/90">{inc.description}</p>}
      {actionError && <div className="mt-4"><ErrorNote message={actionError} /></div>}

      <div className="mt-8 grid gap-6 lg:grid-cols-5">
        <section className="lg:col-span-3" aria-labelledby="analysis-h">
          <h2 id="analysis-h" className="text-sm font-medium text-muted">Root-cause analysis</h2>
          {!latest ? (
            <Card className="mt-3 p-5 text-sm text-muted">Analysis in progress. Resolve-X is correlating telemetry and recent deployments…</Card>
          ) : (
            <Card className="mt-3 p-5">
              <p className="text-xs uppercase tracking-wider text-accent-2">Probable root cause · {latest.confidence}% confidence</p>
              <p className="mt-2 text-lg leading-snug">{latest.rootCause}</p>
              <p className="mt-1 text-xs text-muted">Analyzed by {latest.analyzer} · {timeAgo(latest.createdAt)}</p>

              <h3 className="mt-6 text-xs uppercase tracking-wider text-muted">Recommended action</h3>
              <p className="mt-1 text-sm">{latest.recommendation.description}</p>
              {latest.recommendation.risk && <p className="mt-1 text-xs text-warn">Risk: {latest.recommendation.risk}</p>}
              {latest.approval === "pending" && (
                <div className="mt-4 flex items-center gap-2">
                  <button className={btn} onClick={() => act(`/api/v1/analyses/${latest.id}/approve`, { method: "POST" })}>Approve</button>
                  <button className={btnGhost} onClick={() => act(`/api/v1/analyses/${latest.id}/reject`, { method: "POST" })}>Reject</button>
                  <span className="text-xs text-muted">Resolve-X records your decision. It does not change your systems.</span>
                </div>
              )}
              {(latest.approval === "approved" || latest.approval === "rejected") && (
                <p className="mt-4 text-sm text-muted">Remediation {latest.approval} by {latest.decidedBy}.</p>
              )}

              <h3 className="mt-6 text-xs uppercase tracking-wider text-muted">Evidence</h3>
              <ul className="mt-2 space-y-2">
                {latest.evidence.map((e, i) => (
                  <li key={i} className="flex gap-3 text-sm">
                    <span className="font-mono text-xs text-muted">{fmtTime(e.at)}</span>
                    <span className={`w-20 shrink-0 font-mono text-xs uppercase ${kindTone[e.kind] ?? "text-muted"}`}>{e.kind}</span>
                    <span className="min-w-0 break-words text-fg/90">{e.summary}</span>
                  </li>
                ))}
              </ul>
            </Card>
          )}
        </section>

        <section className="lg:col-span-2" aria-labelledby="timeline-h">
          <h2 id="timeline-h" className="text-sm font-medium text-muted">Timeline</h2>
          <Card className="mt-3 p-5">
            <ol className="space-y-4 border-l border-line pl-4">
              {inc.timeline.map((t, i) => (
                <li key={i} className="relative text-sm">
                  <span className="absolute -left-[21px] top-1.5 h-2 w-2 rounded-full bg-accent" />
                  <p className="font-mono text-xs text-muted">{fmtTime(t.at)} · {t.kind}</p>
                  <p className="mt-0.5">{t.summary}</p>
                </li>
              ))}
            </ol>
          </Card>
        </section>
      </div>
    </main>
  );
}
