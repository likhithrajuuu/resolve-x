"use client";

import type { ReactNode } from "react";
import type { Incident } from "@/lib/api";

export function Card({ children, className = "" }: { children: ReactNode; className?: string }) {
  return <div className={`rounded-xl border border-line bg-surface ${className}`}>{children}</div>;
}

export function PageHeader({ title, children }: { title: string; children?: ReactNode }) {
  return (
    <div className="flex flex-wrap items-center justify-between gap-3">
      <h1 className="text-2xl font-semibold tracking-tight">{title}</h1>
      <div className="flex flex-wrap items-center gap-2">{children}</div>
    </div>
  );
}

const sevTone = { "SEV-1": "bg-danger/15 text-danger", "SEV-2": "bg-warn/15 text-warn", "SEV-3": "bg-accent/15 text-accent" } as const;
export const SeverityBadge = ({ s }: { s: Incident["severity"] }) => (
  <span className={`w-fit rounded px-2 py-0.5 text-xs font-medium ${sevTone[s]}`}>{s}</span>
);

const statusTone = { open: "text-muted", investigating: "text-warn", identified: "text-accent", resolved: "text-accent-2" } as const;
export const StatusLabel = ({ s }: { s: Incident["status"] }) => <span className={`text-sm capitalize ${statusTone[s]}`}>{s}</span>;

export function Empty({ children }: { children: ReactNode }) {
  return <div className="px-4 py-10 text-center text-sm text-muted">{children}</div>;
}

export function ErrorNote({ message }: { message: string }) {
  return <div role="alert" className="rounded-md border border-danger/40 bg-danger/10 px-3 py-2 text-sm text-danger">{message}</div>;
}

export function Loading() {
  return <div className="px-4 py-10 text-center text-sm text-muted">Loading…</div>;
}

export function WindowSelect({ value, onChange }: { value: string; onChange: (v: string) => void }) {
  return (
    <select
      aria-label="Time window"
      value={value}
      onChange={(e) => onChange(e.target.value)}
      className="rounded-md border border-line bg-surface px-2.5 py-1.5 text-sm outline-none focus:border-accent"
    >
      {[["15m", "Last 15 min"], ["1h", "Last hour"], ["6h", "Last 6 hours"], ["24h", "Last 24 hours"], ["168h", "Last 7 days"]].map(([v, l]) => (
        <option key={v} value={v}>{l}</option>
      ))}
    </select>
  );
}

export const btn = "rounded-md bg-accent px-3.5 py-2 text-sm font-medium text-bg transition-opacity hover:opacity-90 disabled:opacity-50";
export const btnGhost = "rounded-md border border-line px-3.5 py-2 text-sm transition-colors hover:bg-surface-2 disabled:opacity-50";
export const input = "w-full rounded-md border border-line bg-bg px-3 py-2 text-sm outline-none focus:border-accent";
