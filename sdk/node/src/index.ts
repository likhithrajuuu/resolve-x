import { context, diag, DiagConsoleLogger, DiagLogLevel, trace } from "@opentelemetry/api";
import { logs, SeverityNumber, type AnyValueMap } from "@opentelemetry/api-logs";
import { getNodeAutoInstrumentations } from "@opentelemetry/auto-instrumentations-node";
import { OTLPLogExporter } from "@opentelemetry/exporter-logs-otlp-proto";
import { OTLPMetricExporter } from "@opentelemetry/exporter-metrics-otlp-proto";
import { OTLPTraceExporter } from "@opentelemetry/exporter-trace-otlp-proto";
import { HostMetrics } from "@opentelemetry/host-metrics";
import { metrics } from "@opentelemetry/api";
import { resourceFromAttributes } from "@opentelemetry/resources";
import { BatchLogRecordProcessor } from "@opentelemetry/sdk-logs";
import { PeriodicExportingMetricReader } from "@opentelemetry/sdk-metrics";
import { NodeSDK } from "@opentelemetry/sdk-node";
import { resolve, type Options, type Resolved } from "./config";

export type { Options } from "./config";

let sdk: NodeSDK | undefined;
let current: Resolved | undefined;
const restore: Array<() => void> = [];

/**
 * Starts tracing, metrics and logs and sends them to Resolve-X.
 * Call once, as early as possible (before importing the libraries you want
 * instrumented), or use `node -r @resolve-x/sdk/register app.js`.
 */
export function init(options: Options = {}): void {
  if (sdk) return; // idempotent
  const cfg = resolve(options);
  current = cfg;
  const headers = { "x-resolvex-key": cfg.apiKey };

  if (process.env.RESOLVEX_DEBUG === "true") diag.setLogger(new DiagConsoleLogger(), DiagLogLevel.INFO);

  const resource = resourceFromAttributes({
    "service.name": cfg.serviceName,
    "deployment.environment": cfg.environment,
    ...(cfg.version ? { "service.version": cfg.version } : {}),
    "telemetry.distro.name": "resolve-x",
    "telemetry.distro.version": "0.1.0",
    ...cfg.resourceAttributes,
  });

  sdk = new NodeSDK({
    resource,
    traceExporter: new OTLPTraceExporter({ url: `${cfg.endpoint}/v1/traces`, headers }),
    metricReader: new PeriodicExportingMetricReader({
      exporter: new OTLPMetricExporter({ url: `${cfg.endpoint}/v1/metrics`, headers }),
      exportIntervalMillis: cfg.metricIntervalMs,
    }),
    logRecordProcessors: [new BatchLogRecordProcessor({ exporter: new OTLPLogExporter({ url: `${cfg.endpoint}/v1/logs`, headers }) })],
    instrumentations: cfg.autoInstrument
      ? [getNodeAutoInstrumentations({ "@opentelemetry/instrumentation-fs": { enabled: false } })]
      : [],
  });
  sdk.start();

  if (cfg.hostMetrics) new HostMetrics({ meterProvider: metrics.getMeterProvider() as never, name: "resolvex-host" }).start();
  if (cfg.captureConsole) captureConsole();

  // Flush on normal exit and on SIGTERM so the last seconds of data are not lost.
  const flush = () => void shutdown();
  process.once("SIGTERM", flush);
  process.once("beforeExit", flush);

  if (cfg.markDeployment && cfg.version) void markDeployment().catch((e) => diag.warn(`[resolvex] deployment marker failed: ${e}`));
}

/** Flushes buffered telemetry and stops the SDK. Safe to call more than once. */
export async function shutdown(): Promise<void> {
  restore.splice(0).forEach((fn) => fn());
  const s = sdk;
  sdk = undefined;
  if (s) await s.shutdown().catch((e) => diag.warn(`[resolvex] shutdown: ${e}`));
}

/**
 * Tells Resolve-X that `version` of this service was just deployed, so incident
 * analysis can connect a degradation to a release.
 */
export async function markDeployment(opts: { service?: string; version?: string; environment?: string } = {}): Promise<void> {
  if (!current) throw new Error("[resolvex] call init() first");
  const body = {
    service: opts.service ?? current.serviceName,
    version: opts.version ?? current.version,
    environment: opts.environment ?? current.environment,
  };
  if (!body.version) throw new Error("[resolvex] markDeployment needs a version");
  const res = await fetch(`${current.apiUrl}/api/v1/webhooks/deployments`, {
    method: "POST",
    headers: { "content-type": "application/json", "x-resolvex-key": current.apiKey },
    body: JSON.stringify(body),
  });
  if (!res.ok) throw new Error(`[resolvex] deployment marker rejected: HTTP ${res.status}`);
}

type Level = "debug" | "info" | "warn" | "error";
const severity: Record<Level, [SeverityNumber, string]> = {
  debug: [SeverityNumber.DEBUG, "DEBUG"],
  info: [SeverityNumber.INFO, "INFO"],
  warn: [SeverityNumber.WARN, "WARN"],
  error: [SeverityNumber.ERROR, "ERROR"],
};

function emit(level: Level, body: string, attributes?: AnyValueMap): void {
  const [num, text] = severity[level];
  // Passing the active context is what links the record to the current trace.
  logs.getLogger("resolvex").emit({ severityNumber: num, severityText: text, body, attributes, context: context.active() });
}

/** Structured logger whose records are linked to the active trace. */
export const logger = {
  debug: (msg: string, attrs?: AnyValueMap) => emit("debug", msg, attrs),
  info: (msg: string, attrs?: AnyValueMap) => emit("info", msg, attrs),
  warn: (msg: string, attrs?: AnyValueMap) => emit("warn", msg, attrs),
  error: (msg: string, attrs?: AnyValueMap) => emit("error", msg, attrs),
};

/** Runs `fn` inside a named span; records exceptions and sets error status. */
export async function withSpan<T>(name: string, fn: () => Promise<T> | T, attributes?: Record<string, string | number | boolean>): Promise<T> {
  const tracer = trace.getTracer("resolvex");
  return tracer.startActiveSpan(name, { attributes }, async (span) => {
    try {
      return await fn();
    } catch (e) {
      span.recordException(e as Error);
      span.setStatus({ code: 2, message: e instanceof Error ? e.message : String(e) });
      throw e;
    } finally {
      span.end();
    }
  });
}

function captureConsole(): void {
  for (const [method, level] of [["log", "info"], ["info", "info"], ["warn", "warn"], ["error", "error"], ["debug", "debug"]] as const) {
    const original = console[method].bind(console);
    console[method] = (...args: unknown[]) => {
      original(...args);
      try {
        emit(level, args.map((a) => (typeof a === "string" ? a : JSON.stringify(a))).join(" "));
      } catch {
        /* never let telemetry break the app */
      }
    };
    restore.push(() => { console[method] = original; });
  }
}
