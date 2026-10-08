import Link from "next/link";
import { IncidentPreview } from "@/components/IncidentPreview";
import { Logo } from "@/components/Logo";
import { SiteNav } from "@/components/SiteNav";

const features = [
  ["Automatic service discovery", "Services and their dependencies are built from OpenTelemetry and Kubernetes metadata. Nobody maintains a catalog by hand."],
  ["One timeline per incident", "Logs, metrics, traces, deployments and config changes are correlated into a single evidence timeline."],
  ["Root cause, with evidence", "Every conclusion links to the signals behind it, so engineers can check the reasoning instead of trusting a black box."],
  ["Humans approve every fix", "Resolve-X recommends remediation. Nothing runs against your systems until someone on your team approves it."],
  ["OpenTelemetry native", "Standard OTLP over gRPC or HTTP. Java, Python, Go and Node.js work without a proprietary agent."],
  ["Strict tenant isolation", "Tenant identity is derived from your credentials and carried on every event, from the edge to storage."],
] as const;

const steps = [
  ["Connect", "Point your OpenTelemetry exporter or collector at Resolve-X with an API key."],
  ["Discover", "Services, environments and the dependencies between them appear on their own."],
  ["Correlate", "When an alert fires, related signals and recent changes are gathered around it."],
  ["Resolve", "You get a probable root cause, the evidence, and a recommended action to approve."],
] as const;

const scale = [
  ["Stateless ingestion", "Any instance serves any request. Add replicas to add capacity."],
  ["Kafka between every stage", "Slow consumers never slow ingestion. Spikes are absorbed and drained, and data can be replayed."],
  ["Columnar analytics store", "Telemetry lands in ClickHouse, partitioned by day and ordered by tenant, apart from transactional data."],
  ["Backpressure, not data loss", "If the pipeline is saturated, collectors get a retryable error instead of silently dropped data."],
] as const;

const tiers = [
  { name: "Starter", price: "Free", note: "For trying it on one project", items: ["1 project", "Up to 5 services", "3-day retention", "Community support"], cta: "Start free" },
  { name: "Team", price: "Contact us", note: "For teams running production", items: ["Unlimited projects", "30-day retention", "AI root-cause analysis", "Slack and webhook alerts", "SSO"], cta: "Talk to us", featured: true },
  { name: "Enterprise", price: "Custom", note: "For high-volume platforms", items: ["Dedicated capacity", "Custom retention", "Private networking", "Audit log export", "Priority support"], cta: "Talk to us" },
] as const;

const faqs = [
  ["Do I need to change my application code?", "No, if you already use OpenTelemetry. Otherwise add the standard OTel SDK for your language and export to Resolve-X."],
  ["Will it take actions on my infrastructure by itself?", "No. Remediation is always a recommendation that a person approves first."],
  ["Where does my telemetry go?", "Into your tenant's isolated storage. Tenant identity is checked at the edge and carried through every stage."],
  ["Which LLM does it use?", "Analysis runs behind an abstraction so the model can be swapped. We send evidence sets, not raw firehose data."],
] as const;

export default function Home() {
  return (
    <>
      <SiteNav />
      <main>
        <section className="glow relative overflow-hidden">
          <div className="grid-bg absolute inset-0" aria-hidden />
          <div className="relative mx-auto max-w-6xl px-4 pb-20 pt-20 sm:px-6 sm:pt-28">
            <div className="mx-auto max-w-3xl text-center">
              <p className="mx-auto inline-block rounded-full border border-line bg-surface/70 px-3 py-1 text-xs text-muted">
                Early access · Built on OpenTelemetry
              </p>
              <h1 className="mt-6 text-balance text-4xl font-semibold tracking-tight sm:text-6xl">
                When production breaks, know <span className="text-accent">why</span> in minutes.
              </h1>
              <p className="mx-auto mt-5 max-w-2xl text-pretty text-lg text-muted">
                Resolve-X correlates logs, metrics, traces and deployments across your services, then tells your
                team the probable root cause and what to do next.
              </p>
              <div className="mt-8 flex flex-col items-center justify-center gap-3 sm:flex-row">
                <a href="#cta" className="w-full rounded-md bg-accent px-5 py-2.5 text-sm font-medium text-bg transition-opacity hover:opacity-90 sm:w-auto">
                  Get early access
                </a>
                <Link href="/dashboard" className="w-full rounded-md border border-line px-5 py-2.5 text-sm transition-colors hover:bg-surface sm:w-auto">
                  See the live demo
                </Link>
              </div>
            </div>
            <div className="mx-auto mt-14 max-w-4xl">
              <IncidentPreview />
              <p className="mt-3 text-center text-xs text-muted">Illustration with sample data.</p>
            </div>
          </div>
        </section>

        <section className="mx-auto max-w-6xl px-4 py-20 sm:px-6">
          <h2 className="max-w-2xl text-balance text-3xl font-semibold tracking-tight">
            Incidents are slow because the evidence is scattered.
          </h2>
          <p className="mt-4 max-w-2xl text-muted">
            Engineers open five tools, search logs, follow traces, check what deployed, and guess. In a system with
            dozens of services, that investigation is most of the outage.
          </p>
        </section>

        <section id="product" className="mx-auto max-w-6xl scroll-mt-16 px-4 pb-20 sm:px-6">
          <h2 className="text-3xl font-semibold tracking-tight">What you get</h2>
          <div className="mt-10 grid gap-px overflow-hidden rounded-xl border border-line bg-line sm:grid-cols-2 lg:grid-cols-3">
            {features.map(([title, body]) => (
              <div key={title} className="bg-surface p-6">
                <h3 className="font-medium">{title}</h3>
                <p className="mt-2 text-sm leading-relaxed text-muted">{body}</p>
              </div>
            ))}
          </div>
        </section>

        <section id="how" className="scroll-mt-16 border-y border-line bg-surface/50">
          <div className="mx-auto max-w-6xl px-4 py-20 sm:px-6">
            <h2 className="text-3xl font-semibold tracking-tight">How it works</h2>
            <ol className="mt-10 grid gap-6 sm:grid-cols-2 lg:grid-cols-4">
              {steps.map(([title, body], i) => (
                <li key={title}>
                  <span className="font-mono text-sm text-accent">0{i + 1}</span>
                  <h3 className="mt-2 font-medium">{title}</h3>
                  <p className="mt-2 text-sm leading-relaxed text-muted">{body}</p>
                </li>
              ))}
            </ol>
          </div>
        </section>

        <section id="scale" className="mx-auto max-w-6xl scroll-mt-16 px-4 py-20 sm:px-6">
          <h2 className="text-3xl font-semibold tracking-tight">Built for traffic spikes</h2>
          <p className="mt-4 max-w-2xl text-muted">
            Incidents are exactly when telemetry volume jumps. The pipeline is designed so that load on one stage
            never takes down another.
          </p>
          <div className="mt-10 grid gap-4 sm:grid-cols-2">
            {scale.map(([title, body]) => (
              <div key={title} className="rounded-xl border border-line bg-surface p-6">
                <h3 className="font-medium">{title}</h3>
                <p className="mt-2 text-sm leading-relaxed text-muted">{body}</p>
              </div>
            ))}
          </div>
        </section>

        <section id="pricing" className="scroll-mt-16 border-t border-line bg-surface/50">
          <div className="mx-auto max-w-6xl px-4 py-20 sm:px-6">
            <h2 className="text-3xl font-semibold tracking-tight">Pricing</h2>
            <p className="mt-4 max-w-2xl text-muted">Early-access plans. Final pricing will be announced before general availability.</p>
            <div className="mt-10 grid gap-4 lg:grid-cols-3">
              {tiers.map((t) => (
                <div key={t.name} className={`flex flex-col rounded-xl border p-6 ${"featured" in t ? "border-accent bg-surface" : "border-line bg-surface"}`}>
                  <h3 className="font-medium">{t.name}</h3>
                  <p className="mt-3 text-3xl font-semibold">{t.price}</p>
                  <p className="mt-1 text-sm text-muted">{t.note}</p>
                  <ul className="mt-6 flex-1 space-y-2 text-sm">
                    {t.items.map((it) => (
                      <li key={it} className="flex gap-2">
                        <span className="text-accent-2" aria-hidden>✓</span>
                        {it}
                      </li>
                    ))}
                  </ul>
                  <a href="#cta" className={`mt-8 rounded-md px-4 py-2 text-center text-sm font-medium ${"featured" in t ? "bg-accent text-bg" : "border border-line hover:bg-surface-2"}`}>
                    {t.cta}
                  </a>
                </div>
              ))}
            </div>
          </div>
        </section>

        <section id="faq" className="mx-auto max-w-3xl scroll-mt-16 px-4 py-20 sm:px-6">
          <h2 className="text-3xl font-semibold tracking-tight">Questions</h2>
          <div className="mt-8 divide-y divide-line border-y border-line">
            {faqs.map(([q, a]) => (
              <details key={q} className="group py-4">
                <summary className="flex cursor-pointer list-none items-center justify-between font-medium">
                  {q}
                  <span className="text-muted transition-transform group-open:rotate-45" aria-hidden>+</span>
                </summary>
                <p className="mt-3 text-sm leading-relaxed text-muted">{a}</p>
              </details>
            ))}
          </div>
        </section>

        <section id="cta" className="glow scroll-mt-16 border-t border-line">
          <div className="mx-auto max-w-2xl px-4 py-24 text-center sm:px-6">
            <h2 className="text-balance text-3xl font-semibold tracking-tight sm:text-4xl">Get early access</h2>
            <p className="mt-3 text-muted">Tell us where to reach you. We&apos;ll invite teams in batches.</p>
            <form action="mailto:hello@resolve-x.dev" method="post" encType="text/plain" className="mx-auto mt-8 flex max-w-md flex-col gap-2 sm:flex-row">
              <label htmlFor="email" className="sr-only">Work email</label>
              <input id="email" name="email" type="email" required placeholder="you@company.com" className="min-w-0 flex-1 rounded-md border border-line bg-surface px-3 py-2.5 text-sm outline-none focus:border-accent" />
              <button className="rounded-md bg-accent px-5 py-2.5 text-sm font-medium text-bg hover:opacity-90">Request access</button>
            </form>
          </div>
        </section>
      </main>
      <footer className="border-t border-line">
        <div className="mx-auto flex max-w-6xl flex-col items-center justify-between gap-3 px-4 py-8 text-sm text-muted sm:flex-row sm:px-6">
          <Logo />
          <p>© Resolve-X</p>
        </div>
      </footer>
    </>
  );
}
