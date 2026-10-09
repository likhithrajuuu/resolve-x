import Link from "next/link";
import { Logo } from "@/components/Logo";

export default function NotFound() {
  return (
    <main className="glow grid min-h-screen place-items-center px-4 text-center">
      <div>
        <Link href="/" className="inline-block" aria-label="Resolve-X home"><Logo /></Link>
        <h1 className="mt-8 text-4xl font-semibold tracking-tight">Page not found</h1>
        <p className="mt-2 text-muted">That page doesn&apos;t exist or has moved.</p>
        <div className="mt-6 flex justify-center gap-3 text-sm">
          <Link href="/" className="rounded-md border border-line px-4 py-2 hover:bg-surface">Home</Link>
          <Link href="/dashboard" className="rounded-md bg-accent px-4 py-2 font-medium text-bg">Dashboard</Link>
        </div>
      </div>
    </main>
  );
}
