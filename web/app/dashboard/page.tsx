import { IncidentPreview } from "@/components/IncidentPreview";
import { incidents, kpis } from "@/lib/demo";

const sev = { "SEV-1": "bg-danger/15 text-danger", "SEV-2": "bg-warn/15 text-warn", "SEV-3": "bg-accent/15 text-accent" } as const;
const status = { Investigating: "text-warn", Identified: "text-accent", Resolved: "text-accent-2" } as const;

export const metadata = { title: "Incidents — Resolve-X" };

export default function Dashboard() {
  return (
    <main className="mx-auto max-w-5xl px-4 py-8 sm:px-6">
      <h1 className="text-2xl font-semibold tracking-tight">Incidents</h1>
      <dl className="mt-6 grid grid-cols-2 gap-3 lg:grid-cols-4">
        {kpis.map((k) => (
          <div key={k.label} className="rounded-lg border border-line bg-surface p-4">
            <dt className="text-xs text-muted">{k.label}</dt>
            <dd className="mt-1 text-xl font-semibold">{k.value}</dd>
          </div>
        ))}
      </dl>

      <ul className="mt-8 divide-y divide-line overflow-hidden rounded-xl border border-line bg-surface">
        {incidents.map((i) => (
          <li key={i.id} className="flex flex-col gap-2 p-4 sm:flex-row sm:items-center sm:gap-4">
            <span className={`w-fit rounded px-2 py-0.5 text-xs font-medium ${sev[i.severity]}`}>{i.severity}</span>
            <div className="min-w-0 flex-1">
              <p className="truncate font-medium">
                <span className="mr-2 font-mono text-xs text-muted">{i.id}</span>{i.title}
              </p>
              <p className="mt-0.5 truncate text-sm text-muted">
                {i.service} · {i.cause}{i.confidence ? ` (${i.confidence}%)` : ""}
              </p>
            </div>
            <div className="text-sm sm:text-right">
              <p className={status[i.status]}>{i.status}</p>
              <p className="text-xs text-muted">{i.opened}</p>
            </div>
          </li>
        ))}
      </ul>

      <h2 className="mt-10 text-sm font-medium text-muted">Latest analysis · INC-482</h2>
      <div className="mt-3"><IncidentPreview /></div>
    </main>
  );
}
