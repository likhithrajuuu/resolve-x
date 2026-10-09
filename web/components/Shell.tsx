"use client";

import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { useEffect, useState, type ReactNode } from "react";
import { api, session } from "@/lib/api";
import { Logo } from "./Logo";

const nav = [
  ["/dashboard", "Incidents"],
  ["/dashboard/services", "Services"],
  ["/dashboard/dependencies", "Dependencies"],
  ["/dashboard/traces", "Traces"],
  ["/dashboard/logs", "Logs"],
  ["/dashboard/settings", "Settings"],
] as const;

export function Shell({ children }: { children: ReactNode }) {
  const path = usePathname();
  const router = useRouter();
  const [email, setEmail] = useState<string | null>(null);

  useEffect(() => {
    if (!session.get()) {
      router.replace("/login");
      return;
    }
    api<{ email: string }>("/api/v1/auth/me").then((m) => setEmail(m.email)).catch(() => {});
  }, [router]);

  const active = (href: string) => (href === "/dashboard" ? path === href || path.startsWith("/dashboard/incidents") : path.startsWith(href));

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
        <nav className="mt-3 flex gap-1 overflow-x-auto text-sm md:mt-8 md:flex-col" aria-label="Dashboard">
          {nav.map(([href, label]) => (
            <Link
              key={href}
              href={href}
              aria-current={active(href) ? "page" : undefined}
              className={`whitespace-nowrap rounded-md px-3 py-2 ${active(href) ? "bg-surface-2 text-fg" : "text-muted hover:text-fg"}`}
            >{label}</Link>
          ))}
        </nav>
        <div className="mt-8 hidden text-xs text-muted md:block">
          <p className="truncate">{email ?? "…"}</p>
          <button className="mt-1 hover:text-fg" onClick={() => { session.clear(); router.replace("/login"); }}>Sign out</button>
        </div>
      </aside>
      <div className="min-w-0 flex-1">{children}</div>
    </div>
  );
}
