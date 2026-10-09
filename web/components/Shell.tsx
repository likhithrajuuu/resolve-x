"use client";

import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { useEffect, useState, type ReactNode } from "react";
import { api, session } from "@/lib/api";
import { Logo } from "./Logo";
import { CommandPalette } from "./CommandPalette";

const nav: { label: string; items: [string, string][] }[] = [
  { label: "Monitor", items: [["/dashboard", "Overview"], ["/dashboard/incidents", "Incidents"], ["/dashboard/services", "Services"], ["/dashboard/dependencies", "Dependencies"]] },
  { label: "Explore", items: [["/dashboard/traces", "Traces"], ["/dashboard/logs", "Logs"], ["/dashboard/metrics", "Metrics"]] },
  { label: "Build", items: [["/dashboard/dashboards", "Dashboards"], ["/dashboard/alerts", "Alert rules"], ["/dashboard/slos", "SLOs"]] },
  { label: "Account", items: [["/dashboard/settings", "Settings"]] },
];

export function Shell({ children }: { children: ReactNode }) {
  const path = usePathname();
  const router = useRouter();
  const [email, setEmail] = useState<string | null>(null);
  const [palette, setPalette] = useState(false);

  useEffect(() => {
    if (!session.get()) {
      router.replace("/login");
      return;
    }
    api<{ email: string }>("/api/v1/auth/me").then((m) => setEmail(m.email)).catch(() => {});
  }, [router]);

  const active = (href: string) => (href === "/dashboard" ? path === href : path.startsWith(href));

  return (
    <div className="flex min-h-screen flex-col md:flex-row">
      <aside className="shrink-0 border-b border-line bg-surface/50 p-3 md:w-56 md:border-b-0 md:border-r md:p-4">
        <div className="flex items-center justify-between">
          <Link href="/" aria-label="Resolve-X home"><Logo /></Link>
          <button
            className="text-xs text-muted hover:text-fg md:hidden"
            onClick={() => { session.clear(); router.replace("/login"); }}
          >Sign out</button>
        </div>
        <button onClick={() => setPalette(true)} className="mt-4 flex w-full items-center justify-between rounded-md border border-line px-3 py-2 text-sm text-muted hover:bg-surface-2" aria-label="Open command palette">
          <span>Search…</span><kbd className="rounded border border-line px-1.5 text-xs">⌘K</kbd>
        </button>
        <nav className="mt-3 flex gap-1 overflow-x-auto text-sm md:mt-5 md:flex-col md:gap-0" aria-label="Dashboard">
          {nav.map((g) => (
            <div key={g.label} className="flex gap-1 md:mb-3 md:flex-col">
              <p className="hidden px-3 pb-1 text-[11px] uppercase tracking-wider text-muted/70 md:block">{g.label}</p>
              {g.items.map(([href, label]) => (
                <Link key={href} href={href} aria-current={active(href) ? "page" : undefined}
                  className={`whitespace-nowrap rounded-md px-3 py-1.5 ${active(href) ? "bg-surface-2 text-fg" : "text-muted hover:text-fg"}`}>{label}</Link>
              ))}
            </div>
          ))}
        </nav>
        <div className="mt-8 hidden text-xs text-muted md:block">
          <p className="truncate">{email ?? "…"}</p>
          <button className="mt-1 hover:text-fg" onClick={() => { session.clear(); router.replace("/login"); }}>Sign out</button>
        </div>
      </aside>
      <div className="min-w-0 flex-1">{children}</div>
      <CommandPalette open={palette} onOpenChange={setPalette} />
    </div>
  );
}
