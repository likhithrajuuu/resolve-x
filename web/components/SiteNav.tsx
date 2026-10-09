import Link from "next/link";
import { Logo } from "./Logo";

const links = [
  ["Product", "#product"],
  ["How it works", "#how"],
  ["Scale", "#scale"],
  ["Pricing", "#pricing"],
  ["FAQ", "#faq"],
] as const;

export function SiteNav() {
  return (
    <header className="sticky top-0 z-30 border-b border-line/70 bg-bg/80 backdrop-blur">
      <nav className="mx-auto flex h-14 max-w-6xl items-center justify-between px-4 sm:px-6">
        <Link href="/" aria-label="Resolve-X home">
          <Logo />
        </Link>
        <ul className="hidden items-center gap-7 text-sm text-muted md:flex">
          {links.map(([label, href]) => (
            <li key={href}>
              <a href={href} className="transition-colors hover:text-fg">
                {label}
              </a>
            </li>
          ))}
        </ul>
        <div className="flex items-center gap-2">
          <Link href="/login" className="px-3 py-1.5 text-sm text-muted transition-colors hover:text-fg">
            Sign in
          </Link>
          <Link href="/signup" className="rounded-md bg-fg px-3.5 py-1.5 text-sm font-medium text-bg transition-opacity hover:opacity-90">
            Start free
          </Link>
        </div>
      </nav>
    </header>
  );
}
