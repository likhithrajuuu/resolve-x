"use client";

import { useState, type FormEvent } from "react";
import { api, useApi, type Team } from "@/lib/api";
import { btn, btnGhost, Card, ErrorNote, input, Loading, PageHeader } from "@/components/ui";

type LinkInfo = { kind: "invite" | "reset"; email: string; url: string };

export default function TeamPage() {
  const { data, error, loading, reload } = useApi<Team>("/api/v1/members");
  const [link, setLink] = useState<LinkInfo | null>(null);
  const [err, setErr] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);

  async function run(fn: () => Promise<void>) {
    setErr(null);
    try { await fn(); await reload(); } catch (e) { setErr(e instanceof Error ? e.message : "Failed"); }
  }

  async function invite(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const f = new FormData(e.currentTarget);
    const form = e.currentTarget;
    await run(async () => {
      const r = await api<{ token: string; email: string }>("/api/v1/members/invites", { method: "POST", body: JSON.stringify({ email: f.get("email"), role: f.get("role") }) });
      setLink({ kind: "invite", email: r.email, url: `${location.origin}/accept-invite?token=${r.token}` });
      form.reset();
    });
  }

  if (loading && !data) return <Loading />;
  if (!data) return <main className="p-6"><ErrorNote message={error ?? "Could not load the team"} /></main>;
  const me = data.members.find((m) => m.self);

  return (
    <main className="mx-auto max-w-3xl px-4 py-8 sm:px-6">
      <PageHeader title="Team" />
      <p className="mt-2 text-sm text-muted">
        <b className="text-fg">Owner and admins</b> manage keys, alert rules, SLOs, AI and notification settings, and approve remediation. <b className="text-fg">Members</b> can investigate: view all telemetry, use the AI assistant, and build dashboards.
      </p>
      {err && <div className="mt-4"><ErrorNote message={err} /></div>}

      {link && (
        <Card className="mt-6 border-accent-2/40 bg-accent-2/5 p-4">
          <p className="text-sm font-medium text-accent-2">{link.kind === "invite" ? `Invite link for ${link.email}` : `Password reset link for ${link.email}`}</p>
          <p className="mt-1 text-xs text-muted">Send it to them yourself. It is shown only once and works for {link.kind === "invite" ? "7 days" : "24 hours"}.</p>
          <input readOnly value={link.url} onFocus={(e) => e.currentTarget.select()} aria-label="Link" className={`${input} mt-2 font-mono text-xs`} />
          <div className="mt-2 flex gap-2">
            <button className={btnGhost} onClick={async () => { await navigator.clipboard?.writeText(link.url); setCopied(true); setTimeout(() => setCopied(false), 1500); }}>{copied ? "Copied" : "Copy link"}</button>
            <button className={btnGhost} onClick={() => setLink(null)}>Dismiss</button>
          </div>
        </Card>
      )}

      {data.canManage && (
        <Card className="mt-6 p-4">
          <form onSubmit={invite} className="flex flex-wrap items-end gap-3">
            <label className="min-w-[14rem] flex-1 text-sm text-muted">Invite by email<input name="email" type="email" required placeholder="teammate@company.com" className={`${input} mt-1`} /></label>
            <label className="text-sm text-muted">Role<select name="role" className={`${input} mt-1 w-32`}><option value="member">Member</option>{me?.role === "owner" && <option value="admin">Admin</option>}</select></label>
            <button className={btn}>Create invite link</button>
          </form>
        </Card>
      )}

      <h2 className="mt-8 text-sm font-medium text-muted">Members</h2>
      <Card className="mt-3 divide-y divide-line">
        {data.members.map((m) => (
          <div key={m.userId} className="flex flex-wrap items-center gap-3 p-4">
            <div className="min-w-0 flex-1"><p className="truncate font-medium">{m.email}{m.self && <span className="ml-2 text-xs text-muted">you</span>}</p></div>
            <span className="rounded bg-surface-2 px-2 py-0.5 text-xs capitalize">{m.role}</span>
            {data.canManage && !m.self && m.role !== "owner" && (me?.role === "owner" || m.role === "member") && (
              <div className="flex gap-1.5 text-xs">
                {me?.role === "owner" && <button className={btnGhost} onClick={() => run(() => api(`/api/v1/members/${m.userId}`, { method: "PATCH", body: JSON.stringify({ role: m.role === "admin" ? "member" : "admin" }) }))}>Make {m.role === "admin" ? "member" : "admin"}</button>}
                <button className={btnGhost} onClick={() => run(async () => { const r = await api<{ token: string }>(`/api/v1/members/${m.userId}/reset-link`, { method: "POST" }); setLink({ kind: "reset", email: m.email, url: `${location.origin}/reset-password?token=${r.token}` }); })}>Reset password</button>
                <button className={btnGhost} onClick={() => confirm(`Remove ${m.email}? They lose access immediately.`) && run(() => api(`/api/v1/members/${m.userId}`, { method: "DELETE" }))}>Remove</button>
              </div>
            )}
          </div>
        ))}
      </Card>

      {data.pending.length > 0 && (<>
        <h2 className="mt-8 text-sm font-medium text-muted">Pending links</h2>
        <Card className="mt-3 divide-y divide-line">
          {data.pending.map((p) => (
            <div key={p.id} className="flex items-center gap-3 p-4 text-sm">
              <span className="min-w-0 flex-1 truncate">{p.email} <span className="text-muted">· {p.kind === "invite" ? `invited as ${p.role}` : "password reset"} · expires {new Date(p.expiresAt).toLocaleDateString()}</span></span>
              <button className={btnGhost} onClick={() => run(() => api(`/api/v1/members/invites/${p.id}`, { method: "DELETE" }))}>Revoke</button>
            </div>
          ))}
        </Card>
      </>)}
    </main>
  );
}
