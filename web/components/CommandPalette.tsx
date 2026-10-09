"use client";

import { useRouter } from "next/navigation";
import { useEffect, useMemo, useRef, useState } from "react";
import { useApi, type Incident, type ServiceRow } from "@/lib/api";

type Item = { id: string; label: string; hint: string; href: string };

const PAGES: Item[] = [
  ["/dashboard", "Overview"], ["/dashboard/incidents", "Incidents"], ["/dashboard/services", "Services"], ["/dashboard/dependencies", "Dependency map"],
  ["/dashboard/traces", "Traces"], ["/dashboard/logs", "Logs"], ["/dashboard/metrics", "Metrics explorer"], ["/dashboard/dashboards", "Dashboards"],
  ["/dashboard/alerts", "Alert rules"], ["/dashboard/slos", "SLOs"], ["/dashboard/settings", "Settings and API keys"],
].map(([href, label]) => ({ id: href, label, hint: "Go to", href }));

/** Turns plain phrases like "errors in payments" into a filtered view. */
function interpret(q: string, services: string[]): Item[] {
  const lower = q.toLowerCase();
  const svc = services.find((s) => lower.includes(s.toLowerCase()));
  if (!svc) return [];
  const enc = encodeURIComponent(svc);
  const out: Item[] = [];
  if (/(error|fail|exception|crash)/.test(lower)) {
    out.push({ id: "q-logs", label: `Error logs for ${svc}`, hint: "Search", href: `/dashboard/logs?service=${enc}&minSeverity=17` });
    out.push({ id: "q-traces", label: `Failing traces for ${svc}`, hint: "Search", href: `/dashboard/traces?service=${enc}&status=error` });
  }
  if (/(slow|latenc|p9\d)/.test(lower)) out.push({ id: "q-slow", label: `Slow traces for ${svc}`, hint: "Search", href: `/dashboard/traces?service=${enc}` });
  return out;
}

export function CommandPalette({ open, onOpenChange }: { open: boolean; onOpenChange: (v: boolean) => void }) {
  const router = useRouter();
  const [q, setQ] = useState("");
  const [cursor, setCursor] = useState(0);
  const inputRef = useRef<HTMLInputElement>(null);
  const services = useApi<ServiceRow[]>(open ? "/api/v1/services?window=24h" : null);
  const incidents = useApi<Incident[]>(open ? "/api/v1/incidents" : null);

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "k") { e.preventDefault(); onOpenChange(!open); }
      if (e.key === "Escape") onOpenChange(false);
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [open, onOpenChange]);

  useEffect(() => { if (open) inputRef.current?.focus(); }, [open]);

  const items = useMemo(() => {
    const names = (services.data ?? []).map((s) => s.name);
    const all: Item[] = [
      ...interpret(q, names),
      ...PAGES,
      ...names.map((n) => ({ id: "s-" + n, label: n, hint: "Service", href: `/dashboard/services/${encodeURIComponent(n)}` })),
      ...(incidents.data ?? []).slice(0, 15).map((i) => ({ id: i.id, label: i.title, hint: i.severity, href: `/dashboard/incidents/${i.id}` })),
    ];
    const needle = q.trim().toLowerCase();
    if (!needle) return all.slice(0, 12);
    const smart = all.filter((i) => i.id.startsWith("q-"));
    return [...smart, ...all.filter((i) => !i.id.startsWith("q-") && (i.label + " " + i.hint).toLowerCase().includes(needle))].slice(0, 12);
  }, [q, services.data, incidents.data]);

  if (!open) return null;
  const go = (it: Item) => { onOpenChange(false); setQ(""); setCursor(0); router.push(it.href); };

  return (
    <div className="fixed inset-0 z-50 flex items-start justify-center bg-black/60 px-4 pt-[15vh]" onMouseDown={(e) => e.target === e.currentTarget && onOpenChange(false)}>
      <div role="dialog" aria-modal="true" aria-label="Command palette" className="w-full max-w-xl overflow-hidden rounded-xl border border-line bg-surface shadow-2xl">
        <input ref={inputRef} value={q} onChange={(e) => { setQ(e.target.value); setCursor(0); }} placeholder="Jump to a page, service or incident… try “errors in payments”"
          aria-label="Search" className="w-full border-b border-line bg-transparent px-4 py-3.5 text-sm outline-none"
          onKeyDown={(e) => {
            if (e.key === "ArrowDown") { e.preventDefault(); setCursor((c) => Math.min(c + 1, items.length - 1)); }
            if (e.key === "ArrowUp") { e.preventDefault(); setCursor((c) => Math.max(c - 1, 0)); }
            if (e.key === "Enter" && items[cursor]) go(items[cursor]);
          }} />
        <ul role="listbox" className="max-h-80 overflow-y-auto py-1">
          {items.length === 0 && <li className="px-4 py-6 text-center text-sm text-muted">No matches</li>}
          {items.map((it, i) => (
            <li key={it.id} role="option" aria-selected={i === cursor} onMouseEnter={() => setCursor(i)} onClick={() => go(it)}
              className={`flex cursor-pointer items-center justify-between gap-3 px-4 py-2 text-sm ${i === cursor ? "bg-surface-2" : ""}`}>
              <span className="truncate">{it.label}</span><span className="shrink-0 text-xs text-muted">{it.hint}</span>
            </li>
          ))}
        </ul>
      </div>
    </div>
  );
}
