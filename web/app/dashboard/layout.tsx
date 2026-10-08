import Link from "next/link";
import { Logo } from "@/components/Logo";

export default function DashboardLayout({ children }: LayoutProps<"/dashboard">) {
  return (
    <div className="flex min-h-screen">
      <aside className="hidden w-56 shrink-0 border-r border-line bg-surface/50 p-4 md:block">
        <Link href="/" aria-label="Resolve-X home"><Logo /></Link>
        <nav className="mt-8 space-y-1 text-sm">
          {["Incidents", "Services", "Dependencies", "Settings"].map((n, i) => (
            <span key={n} className={`block rounded-md px-3 py-2 ${i === 0 ? "bg-surface-2 text-fg" : "text-muted"}`}>{n}</span>
          ))}
        </nav>
      </aside>
      <div className="min-w-0 flex-1">
        <div className="border-b border-warn/30 bg-warn/10 px-4 py-2 text-center text-xs text-warn">
          Demo with sample data — not connected to a backend yet.
        </div>
        {children}
      </div>
    </div>
  );
}
