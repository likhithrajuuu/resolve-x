"use client";

import Link from "next/link";
import { useSearchParams } from "next/navigation";
import { useCallback, useEffect, useRef, useState, type FormEvent } from "react";
import { api, ApiError, useApi, type AiSettings, type ChatMessage, type ChatStep } from "@/lib/api";
import { Markdown } from "@/components/Markdown";
import { btn, btnGhost, Card, input } from "@/components/ui";

const SUGGESTIONS = [
  "What is unhealthy right now?",
  "What changed before the latest incident?",
  "What are the most common errors in the last hour?",
  "Which service has the worst p95 latency, and why?",
  "Summarise the open incidents for a status update.",
];
const STORE = "rx_chat_v1";

function StepList({ steps }: { steps: ChatStep[] }) {
  return (
    <details className="mt-2 text-xs text-muted">
      <summary className="cursor-pointer select-none">Looked at {steps.length} thing{steps.length > 1 ? "s" : ""}</summary>
      <ul className="mt-1.5 space-y-1">
        {steps.map((s, i) => (
          <li key={i} className="flex gap-2 font-mono">
            <span className={s.ok ? "text-accent-2" : "text-danger"} aria-label={s.ok ? "succeeded" : "failed"}>{s.ok ? "✓" : "✗"}</span>
            <span className="min-w-0 break-all">{s.tool}<span className="text-muted/70"> {s.args === "{}" ? "" : s.args}</span> · {s.durationMs} ms</span>
          </li>
        ))}
      </ul>
    </details>
  );
}

export default function AssistantPage() {
  const params = useSearchParams();
  const incidentId = params.get("incident") ?? undefined;
  const initialQ = params.get("q") ?? "";
  const settings = useApi<AiSettings>("/api/v1/ai/settings");
  const [msgs, setMsgs] = useState<ChatMessage[]>([]);
  const [text, setText] = useState(initialQ);
  const [busy, setBusy] = useState(false);
  const [notConfigured, setNotConfigured] = useState(false);
  const bottom = useRef<HTMLDivElement>(null);
  const sentInitial = useRef(false);

  useEffect(() => {
    // Restoring from sessionStorage can only happen after mount (it does not exist during prerender).
    // eslint-disable-next-line react-hooks/set-state-in-effect
    try { const saved = sessionStorage.getItem(STORE); if (saved && !initialQ) setMsgs(JSON.parse(saved)); } catch { /* ignore */ }
  }, [initialQ]);
  useEffect(() => { try { sessionStorage.setItem(STORE, JSON.stringify(msgs.slice(-30))); } catch { /* ignore */ } bottom.current?.scrollIntoView({ behavior: "smooth", block: "end" }); }, [msgs]);

  const send = useCallback(async (q: string, history: ChatMessage[]) => {
    const next: ChatMessage[] = [...history, { role: "user", content: q }];
    setMsgs(next);
    setBusy(true);
    try {
      const r = await api<{ answer: string; steps: ChatStep[] }>("/api/v1/ai/chat", {
        method: "POST",
        body: JSON.stringify({ messages: next.filter((m) => !m.error).map(({ role, content }) => ({ role, content })), incidentId }),
      });
      setMsgs([...next, { role: "assistant", content: r.answer, steps: r.steps }]);
    } catch (e) {
      if (e instanceof ApiError && e.status === 409) setNotConfigured(true);
      setMsgs([...next, { role: "assistant", content: e instanceof Error ? e.message : "Something went wrong.", error: true }]);
    } finally {
      setBusy(false);
    }
  }, [incidentId]);

  // A question passed in the URL (from the overview or an incident page) is asked once.
  useEffect(() => {
    if (initialQ && !sentInitial.current && settings.data?.configured) { sentInitial.current = true; setText(""); void send(initialQ, []); }
  }, [initialQ, settings.data, send]);

  function submit(e: FormEvent) {
    e.preventDefault();
    const q = text.trim();
    if (!q || busy) return;
    setText("");
    void send(q, msgs);
  }

  const configured = settings.data?.configured && !notConfigured;

  return (
    <main className="mx-auto flex h-[calc(100vh-0px)] max-w-3xl flex-col px-4 py-6 sm:px-6">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">Ask Resolve-X</h1>
          <p className="text-sm text-muted">
            {configured ? <>Answers come from your own telemetry using <span className="text-fg">{settings.data!.provider}</span> · {settings.data!.model}. Read-only.</> : "Ask questions about your services, incidents and deployments."}
            {incidentId && <> Viewing incident <span className="font-mono text-xs">{incidentId}</span>.</>}
          </p>
        </div>
        {msgs.length > 0 && <button className={btnGhost} onClick={() => { setMsgs([]); sessionStorage.removeItem(STORE); }}>New chat</button>}
      </div>

      {settings.data && !configured && (
        <Card className="mt-6 border-accent/40 p-6">
          <h2 className="font-medium">Connect an AI provider</h2>
          <p className="mt-1 text-sm text-muted">Add an API key for Claude, OpenAI (including Codex models), Gemini, or any OpenAI-compatible endpoint such as Ollama. Your key is encrypted and used only for your organization.</p>
          <Link href="/dashboard/settings#ai" className={`${btn} mt-4 inline-block`}>Set up AI in Settings</Link>
        </Card>
      )}

      <div className="mt-4 min-h-0 flex-1 space-y-4 overflow-y-auto pr-1" aria-live="polite">
        {msgs.length === 0 && configured && (
          <div className="mt-6">
            <p className="text-sm text-muted">Try asking:</p>
            <div className="mt-3 flex flex-wrap gap-2">
              {SUGGESTIONS.map((s) => <button key={s} onClick={() => void send(s, [])} className="rounded-full border border-line px-3.5 py-1.5 text-sm text-muted transition-colors hover:bg-surface-2 hover:text-fg">{s}</button>)}
            </div>
          </div>
        )}
        {msgs.map((m, i) => (
          <div key={i} className={m.role === "user" ? "flex justify-end" : ""}>
            <div className={`max-w-[92%] rounded-2xl px-4 py-3 ${m.role === "user" ? "bg-accent text-bg" : m.error ? "border border-danger/40 bg-danger/10 text-danger" : "border border-line bg-surface"}`}>
              {m.role === "user" ? <p className="whitespace-pre-wrap text-sm">{m.content}</p> : m.error ? <p className="text-sm">{m.content}</p> : <Markdown text={m.content} />}
              {m.steps && m.steps.length > 0 && <StepList steps={m.steps} />}
            </div>
          </div>
        ))}
        {busy && <div className="flex items-center gap-2 text-sm text-muted"><span className="h-2 w-2 animate-pulse rounded-full bg-accent" />Looking at your telemetry…</div>}
        <div ref={bottom} />
      </div>

      <form onSubmit={submit} className="mt-4 flex gap-2">
        <label htmlFor="q" className="sr-only">Your question</label>
        <input id="q" value={text} onChange={(e) => setText(e.target.value)} disabled={!configured} maxLength={4000} autoComplete="off"
          placeholder={configured ? "Ask about an incident, a service, an error…" : "Connect an AI provider to start"} className={`${input} py-3`} />
        <button className={btn} disabled={!configured || busy || !text.trim()}>Ask</button>
      </form>
      <p className="mt-2 text-center text-xs text-muted">AI can be wrong. Check the evidence it links to before acting.</p>
    </main>
  );
}
