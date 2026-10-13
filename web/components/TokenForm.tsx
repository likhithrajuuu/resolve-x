"use client";

import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { useState, type FormEvent } from "react";
import { api, session } from "@/lib/api";
import { Logo } from "./Logo";
import { btn, ErrorNote, input } from "./ui";

/** Shared page for invite acceptance and password reset: both take a one-time token and a new password. */
export function TokenForm({ mode }: { mode: "invite" | "reset" }) {
  const router = useRouter();
  const token = useSearchParams().get("token") ?? "";
  const [error, setError] = useState<string | null>(null);
  const [done, setDone] = useState(false);
  const [busy, setBusy] = useState(false);
  const invite = mode === "invite";

  async function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const f = new FormData(e.currentTarget);
    if (f.get("password") !== f.get("confirm")) { setError("The passwords do not match."); return; }
    setBusy(true); setError(null);
    try {
      const r = await api<{ token: string } | undefined>(`/api/v1/auth/${invite ? "accept-invite" : "reset-password"}`, { method: "POST", body: JSON.stringify({ token, password: f.get("password") }) });
      if (invite && r) { session.set(r.token); router.replace("/dashboard"); } else setDone(true);
    } catch (err) { setError(err instanceof Error ? err.message : "Something went wrong"); setBusy(false); }
  }

  return (
    <main className="glow grid min-h-screen place-items-center px-4">
      <div className="w-full max-w-sm">
        <div className="mb-8 flex justify-center"><Link href="/" aria-label="Resolve-X home"><Logo /></Link></div>
        {done ? (
          <div className="rounded-xl border border-line bg-surface p-6 text-center">
            <h1 className="text-xl font-semibold">Password updated</h1>
            <p className="mt-2 text-sm text-muted">All your other sessions were signed out.</p>
            <Link href="/login" className={`${btn} mt-5 inline-block`}>Sign in</Link>
          </div>
        ) : (
          <form onSubmit={submit} className="space-y-4 rounded-xl border border-line bg-surface p-6">
            <h1 className="text-xl font-semibold">{invite ? "Join your team" : "Choose a new password"}</h1>
            {!token && <ErrorNote message="This link is missing its token. Ask your admin for a new one." />}
            {error && <ErrorNote message={error} />}
            <div><label htmlFor="password" className="mb-1 block text-sm text-muted">Password</label><input id="password" name="password" type="password" required minLength={10} maxLength={72} autoComplete="new-password" className={input} /><p className="mt-1 text-xs text-muted">At least 10 characters.</p></div>
            <div><label htmlFor="confirm" className="mb-1 block text-sm text-muted">Confirm password</label><input id="confirm" name="confirm" type="password" required autoComplete="new-password" className={input} /></div>
            <button disabled={busy || !token} className={`${btn} w-full`}>{busy ? "Please wait…" : invite ? "Create account" : "Update password"}</button>
          </form>
        )}
      </div>
    </main>
  );
}
