"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useState, type FormEvent } from "react";
import { api, session } from "@/lib/api";
import { btn, ErrorNote, input } from "./ui";
import { Logo } from "./Logo";

export function AuthForm({ mode }: { mode: "login" | "signup" }) {
  const router = useRouter();
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const signup = mode === "signup";

  async function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setBusy(true);
    setError(null);
    const f = new FormData(e.currentTarget);
    try {
      const r = await api<{ token: string }>(`/api/v1/auth/${mode}`, {
        method: "POST",
        body: JSON.stringify({ email: f.get("email"), password: f.get("password"), company: f.get("company") }),
      });
      session.set(r.token);
      router.replace("/dashboard");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Something went wrong");
      setBusy(false);
    }
  }

  return (
    <main className="glow grid min-h-screen place-items-center px-4">
      <div className="w-full max-w-sm">
        <div className="mb-8 flex justify-center"><Link href="/" aria-label="Resolve-X home"><Logo /></Link></div>
        <form onSubmit={submit} className="space-y-4 rounded-xl border border-line bg-surface p-6">
          <h1 className="text-xl font-semibold">{signup ? "Create your account" : "Sign in"}</h1>
          {error && <ErrorNote message={error} />}
          {signup && (
            <div>
              <label htmlFor="company" className="mb-1 block text-sm text-muted">Company</label>
              <input id="company" name="company" autoComplete="organization" className={input} />
            </div>
          )}
          <div>
            <label htmlFor="email" className="mb-1 block text-sm text-muted">Work email</label>
            <input id="email" name="email" type="email" required autoComplete="email" className={input} />
          </div>
          <div>
            <label htmlFor="password" className="mb-1 block text-sm text-muted">Password</label>
            <input id="password" name="password" type="password" required minLength={signup ? 10 : 1} autoComplete={signup ? "new-password" : "current-password"} className={input} />
            {signup && <p className="mt-1 text-xs text-muted">At least 10 characters.</p>}
          </div>
          <button disabled={busy} className={`${btn} w-full`}>{busy ? "Please wait…" : signup ? "Create account" : "Sign in"}</button>
        </form>
        <p className="mt-4 text-center text-sm text-muted">
          {signup ? <>Already have an account? <Link href="/login" className="text-accent">Sign in</Link></> : <>New here? <Link href="/signup" className="text-accent">Create an account</Link></>}
        </p>
      </div>
    </main>
  );
}
