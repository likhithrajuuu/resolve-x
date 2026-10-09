"use client";

import { useCallback, useEffect, useRef, useState } from "react";

export const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8000";
const TOKEN_KEY = "rx_token";

// The session token lives in localStorage so it survives reloads. That is
// adequate for local development; production should move it to an httpOnly
// cookie set by a server route so page scripts cannot read it.
export const session = {
  get: () => (typeof window === "undefined" ? null : localStorage.getItem(TOKEN_KEY)),
  set: (t: string) => localStorage.setItem(TOKEN_KEY, t),
  clear: () => localStorage.removeItem(TOKEN_KEY),
};

export class ApiError extends Error {
  constructor(public status: number, message: string) {
    super(message);
  }
}

export async function api<T = unknown>(path: string, init: RequestInit = {}): Promise<T> {
  const token = session.get();
  const res = await fetch(API_URL + path, {
    ...init,
    headers: {
      ...(init.body ? { "Content-Type": "application/json" } : {}),
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
      ...init.headers,
    },
  });
  if (res.status === 401 && !path.startsWith("/api/v1/auth/login") && !path.startsWith("/api/v1/auth/signup")) {
    session.clear();
    // Full reload on purpose: drops every in-memory copy of the previous session.
    // eslint-disable-next-line @next/next/no-location-assign-relative-destination
    if (typeof window !== "undefined") window.location.href = "/login";
  }
  if (res.status === 204) return undefined as T;
  const body = await res.json().catch(() => ({}));
  if (!res.ok) throw new ApiError(res.status, body.message ?? `Request failed (${res.status})`);
  return body as T;
}

type Result<T> = { path: string; data: T | null; error: string | null };

/** Fetches `path` (null = skip) and optionally polls. */
export function useApi<T>(path: string | null, refreshMs = 0) {
  const [res, setRes] = useState<Result<T> | null>(null);
  const seq = useRef(0);

  const load = useCallback(async () => {
    if (path === null) return;
    const mine = ++seq.current;
    try {
      const data = await api<T>(path);
      if (mine === seq.current) setRes({ path, data, error: null });
    } catch (e) {
      if (mine === seq.current) setRes((prev) => ({ path, data: prev?.path === path ? prev.data : null, error: e instanceof Error ? e.message : "Request failed" }));
    }
  }, [path]);

  useEffect(() => {
    void load();
    if (!refreshMs || path === null) return;
    const id = setInterval(load, refreshMs);
    return () => clearInterval(id);
  }, [load, refreshMs, path]);

  // Results belong to the path that produced them; a stale one is ignored.
  const current = res?.path === path ? res : null;
  return {
    data: current?.data ?? null,
    error: current?.error ?? null,
    loading: path !== null && current === null,
    reload: load,
  };
}

export type Incident = {
  id: string; title: string; service: string; severity: "SEV-1" | "SEV-2" | "SEV-3";
  status: "open" | "investigating" | "identified" | "resolved"; source: string; description: string;
  createdAt: string; updatedAt: string; resolvedAt?: string;
};
export type Evidence = { kind: string; at: string; summary: string };
export type Analysis = {
  id: string; rootCause: string; confidence: number; evidence: Evidence[];
  recommendation: { action: string; description: string; risk?: string };
  analyzer: string; approval: "pending" | "approved" | "rejected" | "none"; decidedBy?: string; decidedAt?: string; createdAt: string;
};
export type IncidentDetail = Incident & { timeline: { at: string; kind: string; summary: string }[]; analyses: Analysis[] };
export type ServiceRow = {
  name: string; spans: number; errors: number; errorRate: number; p50Ms: number; p95Ms: number; p99Ms: number; firstSeen: string; lastSeen: string;
};
export type Point = { t: string; count: number; errors: number; p95Ms: number };
export type Edge = { source: string; target: string; calls: number; errors: number; p95Ms: number; kind: "service" | "external" };
export type TraceRow = { traceId: string; service: string; name: string; start: string; durationMs: number; spans: number; errors: number; services: number };
export type SpanRow = {
  spanId: string; parentSpanId: string; service: string; name: string; kind: number; start: string; durationMs: number;
  statusCode: number; statusMessage: string; attrs: Record<string, string>;
};
export type LogRow = { ts: string; service: string; severityNumber: number; severityText: string; body: string; traceId: string; spanId: string };

export type OpRow = { name: string; calls: number; errors: number; errorRate: number; p50Ms: number; p95Ms: number; p99Ms: number };
export type MetricName = { service: string; name: string; unit: string; kind: string };
export type MetricSeries = { group: string; points: { t: string; value: number }[] };
export type LogVolume = { t: string; error: number; warn: number; info: number };
export type LogPattern = { pattern: string; service: string; count: number; severityNumber: number; firstSeen: string; lastSeen: string; sample: string };
export type Rule = {
  id: string; name: string; kind: "error_rate" | "latency_p95" | "metric"; service: string; metric: string; agg: string; op: ">" | "<";
  threshold: number; windowMinutes: number; severity: Incident["severity"]; enabled: boolean; lastState: "unknown" | "ok" | "firing" | "nodata"; lastValue?: number; lastEvalAt?: string;
};
export type SLO = {
  id: string; name: string; service: string; kind: "availability" | "latency"; objective: number; latencyMs?: number; windowDays: number;
  total: number; bad: number; sli: number; budgetRemaining: number; burnRate1h: number; status: "ok" | "warning" | "breached" | "nodata";
};
export type Deployment = { id: string; service: string; version: string; environment: string; at: string };
export type Widget = { id: string; type: "metric" | "service" | "logs" | "incidents" | "slo"; title: string; span: 1 | 2; config: Record<string, string> };
export type DashboardDef = { id: string; name: string; widgets: Widget[]; updatedAt: string };
