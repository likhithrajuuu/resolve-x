import Link from "next/link";
import type { ReactNode } from "react";

/**
 * Renders the small Markdown subset the assistant uses. Model output is
 * untrusted, so this builds React elements directly (no innerHTML) and only
 * turns links into anchors when they point inside the dashboard.
 */
function inline(text: string, keyBase: string): ReactNode[] {
  const out: ReactNode[] = [];
  const re = /(\*\*[^*]+\*\*|`[^`]+`|\[[^\]]+\]\([^)\s]+\))/g;
  let last = 0, i = 0;
  for (const m of text.matchAll(re)) {
    if (m.index! > last) out.push(text.slice(last, m.index));
    const t = m[0];
    const k = `${keyBase}-${i++}`;
    if (t.startsWith("**")) out.push(<strong key={k}>{t.slice(2, -2)}</strong>);
    else if (t.startsWith("`")) out.push(<code key={k} className="rounded bg-bg px-1 py-0.5 font-mono text-[0.85em]">{t.slice(1, -1)}</code>);
    else {
      const [, label, href] = /\[([^\]]+)\]\(([^)\s]+)\)/.exec(t)!;
      out.push(href.startsWith("/dashboard/") || href === "/dashboard"
        ? <Link key={k} href={href} className="text-accent underline underline-offset-2">{label}</Link>
        : <span key={k}>{label}</span>);
    }
    last = m.index! + t.length;
  }
  if (last < text.length) out.push(text.slice(last));
  return out;
}

export function Markdown({ text }: { text: string }) {
  const blocks: ReactNode[] = [];
  const lines = text.replace(/\r\n/g, "\n").split("\n");
  let i = 0, k = 0;
  while (i < lines.length) {
    const line = lines[i];
    if (line.startsWith("```")) {
      const code: string[] = [];
      i++;
      while (i < lines.length && !lines[i].startsWith("```")) code.push(lines[i++]);
      i++;
      blocks.push(<pre key={k++} className="my-2 overflow-x-auto rounded-md bg-bg p-3 font-mono text-xs">{code.join("\n")}</pre>);
    } else if (/^\s*([-*]|\d+\.)\s+/.test(line)) {
      const ordered = /^\s*\d+\./.test(line);
      const items: string[] = [];
      while (i < lines.length && /^\s*([-*]|\d+\.)\s+/.test(lines[i])) items.push(lines[i++].replace(/^\s*([-*]|\d+\.)\s+/, ""));
      const Tag = ordered ? "ol" : "ul";
      blocks.push(<Tag key={k++} className={`my-2 space-y-1 pl-5 ${ordered ? "list-decimal" : "list-disc"}`}>{items.map((it, j) => <li key={j}>{inline(it, `${k}-${j}`)}</li>)}</Tag>);
    } else if (/^#{1,3}\s/.test(line)) {
      blocks.push(<p key={k++} className="mt-3 font-semibold">{inline(line.replace(/^#{1,3}\s/, ""), `h${k}`)}</p>);
      i++;
    } else if (line.trim() === "") {
      i++;
    } else {
      const para: string[] = [];
      while (i < lines.length && lines[i].trim() !== "" && !lines[i].startsWith("```") && !/^\s*([-*]|\d+\.)\s+/.test(lines[i]) && !/^#{1,3}\s/.test(lines[i])) para.push(lines[i++]);
      blocks.push(<p key={k++} className="my-2 leading-relaxed">{inline(para.join(" "), `p${k}`)}</p>);
    }
  }
  return <div className="text-sm">{blocks}</div>;
}
