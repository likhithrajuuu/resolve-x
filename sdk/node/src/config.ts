export interface Options {
  /** Project API key (create one in Settings). Defaults to $RESOLVEX_API_KEY. */
  apiKey?: string;
  /** Logical service name shown in Resolve-X. Defaults to $RESOLVEX_SERVICE or $OTEL_SERVICE_NAME. */
  serviceName?: string;
  /** e.g. "production". Defaults to $RESOLVEX_ENVIRONMENT or NODE_ENV. */
  environment?: string;
  /** Release version of this build, e.g. a git SHA or semver. Defaults to $RESOLVEX_VERSION. */
  version?: string;
  /** OTLP/HTTP ingestion base URL. Defaults to $RESOLVEX_ENDPOINT or http://localhost:4318. */
  endpoint?: string;
  /** REST API base URL (used for deployment markers). Defaults to $RESOLVEX_API_URL or http://localhost:8000. */
  apiUrl?: string;
  /** Instrument http, express, pg, mysql, redis, grpc, ... automatically. Default true. */
  autoInstrument?: boolean;
  /** Export process CPU/memory metrics. Default true. */
  hostMetrics?: boolean;
  /** Mirror console.log/info/warn/error as correlated log records. Default false. */
  captureConsole?: boolean;
  /** Record this version as a deployment when init() runs. Default false. */
  markDeployment?: boolean;
  /** Metric export interval in ms. Default 15000. */
  metricIntervalMs?: number;
  /** Extra resource attributes. */
  resourceAttributes?: Record<string, string>;
}

export interface Resolved {
  apiKey: string;
  serviceName: string;
  environment: string;
  version?: string;
  endpoint: string;
  apiUrl: string;
  autoInstrument: boolean;
  hostMetrics: boolean;
  captureConsole: boolean;
  markDeployment: boolean;
  metricIntervalMs: number;
  resourceAttributes: Record<string, string>;
}

const trimSlash = (u: string) => u.replace(/\/+$/, "");
const truthy = (v: string | undefined) => v === "1" || v === "true";

export function resolve(o: Options = {}, env: NodeJS.ProcessEnv = process.env): Resolved {
  const apiKey = o.apiKey ?? env.RESOLVEX_API_KEY;
  if (!apiKey) throw new Error("[resolvex] apiKey is required (pass { apiKey } or set RESOLVEX_API_KEY)");
  const serviceName = o.serviceName ?? env.RESOLVEX_SERVICE ?? env.OTEL_SERVICE_NAME;
  if (!serviceName) throw new Error("[resolvex] serviceName is required (pass { serviceName } or set RESOLVEX_SERVICE)");
  return {
    apiKey,
    serviceName,
    environment: o.environment ?? env.RESOLVEX_ENVIRONMENT ?? env.NODE_ENV ?? "development",
    version: o.version ?? env.RESOLVEX_VERSION,
    endpoint: trimSlash(o.endpoint ?? env.RESOLVEX_ENDPOINT ?? "http://localhost:4318"),
    apiUrl: trimSlash(o.apiUrl ?? env.RESOLVEX_API_URL ?? "http://localhost:8000"),
    autoInstrument: o.autoInstrument ?? env.RESOLVEX_AUTO_INSTRUMENT !== "false",
    hostMetrics: o.hostMetrics ?? env.RESOLVEX_HOST_METRICS !== "false",
    captureConsole: o.captureConsole ?? truthy(env.RESOLVEX_CAPTURE_CONSOLE),
    markDeployment: o.markDeployment ?? truthy(env.RESOLVEX_MARK_DEPLOYMENT),
    metricIntervalMs: o.metricIntervalMs ?? 15000,
    resourceAttributes: o.resourceAttributes ?? {},
  };
}
