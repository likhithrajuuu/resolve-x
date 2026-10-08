// A static illustration of the product. Data is sample data, not a real customer.
const evidence = [
  { t: "10:31:58", k: "deploy", v: "payments v2.14.0 rolled out (3/3 pods)", tone: "text-accent" },
  { t: "10:32:11", k: "metric", v: "payments p99 latency 180ms → 4.2s", tone: "text-warn" },
  { t: "10:32:14", k: "trace", v: "92% of slow traces wait on postgres-orders", tone: "text-warn" },
  { t: "10:32:15", k: "log", v: "ERROR connection pool exhausted (max=10)", tone: "text-danger" },
];

export function IncidentPreview() {
  return (
    <div className="overflow-hidden rounded-xl border border-line bg-surface shadow-2xl shadow-black/50">
      <div className="flex items-center gap-2 border-b border-line px-4 py-3">
        <span className="h-2.5 w-2.5 rounded-full bg-danger" />
        <span className="text-sm font-medium">INC-482 · Checkout latency spike</span>
        <span className="ml-auto rounded bg-danger/15 px-2 py-0.5 text-xs font-medium text-danger">SEV-2</span>
      </div>
      <div className="grid gap-px bg-line md:grid-cols-5">
        <div className="bg-surface p-4 md:col-span-3">
          <p className="text-xs uppercase tracking-wider text-muted">Evidence timeline</p>
          <ul className="mt-3 space-y-3">
            {evidence.map((e) => (
              <li key={e.t} className="flex gap-3 text-sm">
                <span className="font-mono text-xs text-muted">{e.t}</span>
                <span className={`w-12 shrink-0 font-mono text-xs uppercase ${e.tone}`}>{e.k}</span>
                <span className="text-fg/90">{e.v}</span>
              </li>
            ))}
          </ul>
        </div>
        <div className="bg-surface-2 p-4 md:col-span-2">
          <p className="text-xs uppercase tracking-wider text-accent-2">Probable root cause · 87%</p>
          <p className="mt-3 text-sm leading-relaxed">
            The 10:31 payments release lowered the database pool size from 50 to 10. Requests now queue on
            connections, which slows checkout.
          </p>
          <p className="mt-4 text-xs uppercase tracking-wider text-muted">Suggested next step</p>
          <p className="mt-1 text-sm text-fg/90">Roll back payments to v2.13.2, or restore pool size to 50.</p>
          <div className="mt-4 flex gap-2">
            <span className="rounded-md bg-accent px-3 py-1.5 text-xs font-medium text-bg">Review &amp; approve</span>
            <span className="rounded-md border border-line px-3 py-1.5 text-xs text-muted">Dismiss</span>
          </div>
        </div>
      </div>
    </div>
  );
}
