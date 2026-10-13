"use client";

import { useSyncExternalStore } from "react";

const subscribe = (cb: () => void) => {
  const obs = new MutationObserver(cb);
  obs.observe(document.documentElement, { attributes: true, attributeFilter: ["data-theme"] });
  return () => obs.disconnect();
};
const snapshot = () => document.documentElement.dataset.theme === "light";

export function ThemeToggle() {
  const light = useSyncExternalStore(subscribe, snapshot, () => false);
  return (
    <button
      className="hover:text-fg"
      aria-label={light ? "Switch to dark theme" : "Switch to light theme"}
      onClick={() => {
        const next = light ? "dark" : "light";
        document.documentElement.dataset.theme = next;
        try { localStorage.setItem("rx_theme", next); } catch { /* storage unavailable */ }
      }}
    >{light ? "Dark mode" : "Light mode"}</button>
  );
}
