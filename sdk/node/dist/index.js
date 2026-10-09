"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
exports.logger = void 0;
exports.init = init;
exports.shutdown = shutdown;
exports.markDeployment = markDeployment;
exports.withSpan = withSpan;
const api_1 = require("@opentelemetry/api");
const api_logs_1 = require("@opentelemetry/api-logs");
const auto_instrumentations_node_1 = require("@opentelemetry/auto-instrumentations-node");
const exporter_logs_otlp_proto_1 = require("@opentelemetry/exporter-logs-otlp-proto");
const exporter_metrics_otlp_proto_1 = require("@opentelemetry/exporter-metrics-otlp-proto");
const exporter_trace_otlp_proto_1 = require("@opentelemetry/exporter-trace-otlp-proto");
const host_metrics_1 = require("@opentelemetry/host-metrics");
const api_2 = require("@opentelemetry/api");
const resources_1 = require("@opentelemetry/resources");
const sdk_logs_1 = require("@opentelemetry/sdk-logs");
const sdk_metrics_1 = require("@opentelemetry/sdk-metrics");
const sdk_node_1 = require("@opentelemetry/sdk-node");
const config_1 = require("./config");
let sdk;
let current;
const restore = [];
/**
 * Starts tracing, metrics and logs and sends them to Resolve-X.
 * Call once, as early as possible (before importing the libraries you want
 * instrumented), or use `node -r @resolve-x/sdk/register app.js`.
 */
function init(options = {}) {
    if (sdk)
        return; // idempotent
    const cfg = (0, config_1.resolve)(options);
    current = cfg;
    const headers = { "x-resolvex-key": cfg.apiKey };
    if (process.env.RESOLVEX_DEBUG === "true")
        api_1.diag.setLogger(new api_1.DiagConsoleLogger(), api_1.DiagLogLevel.INFO);
    const resource = (0, resources_1.resourceFromAttributes)({
        "service.name": cfg.serviceName,
        "deployment.environment": cfg.environment,
        ...(cfg.version ? { "service.version": cfg.version } : {}),
        "telemetry.distro.name": "resolve-x",
        "telemetry.distro.version": "0.1.0",
        ...cfg.resourceAttributes,
    });
    sdk = new sdk_node_1.NodeSDK({
        resource,
        traceExporter: new exporter_trace_otlp_proto_1.OTLPTraceExporter({ url: `${cfg.endpoint}/v1/traces`, headers }),
        metricReader: new sdk_metrics_1.PeriodicExportingMetricReader({
            exporter: new exporter_metrics_otlp_proto_1.OTLPMetricExporter({ url: `${cfg.endpoint}/v1/metrics`, headers }),
            exportIntervalMillis: cfg.metricIntervalMs,
        }),
        logRecordProcessors: [new sdk_logs_1.BatchLogRecordProcessor({ exporter: new exporter_logs_otlp_proto_1.OTLPLogExporter({ url: `${cfg.endpoint}/v1/logs`, headers }) })],
        instrumentations: cfg.autoInstrument
            ? [(0, auto_instrumentations_node_1.getNodeAutoInstrumentations)({ "@opentelemetry/instrumentation-fs": { enabled: false } })]
            : [],
    });
    sdk.start();
    if (cfg.hostMetrics)
        new host_metrics_1.HostMetrics({ meterProvider: api_2.metrics.getMeterProvider(), name: "resolvex-host" }).start();
    if (cfg.captureConsole)
        captureConsole();
    // Flush on normal exit and on SIGTERM so the last seconds of data are not lost.
    const flush = () => void shutdown();
    process.once("SIGTERM", flush);
    process.once("beforeExit", flush);
    if (cfg.markDeployment && cfg.version)
        void markDeployment().catch((e) => api_1.diag.warn(`[resolvex] deployment marker failed: ${e}`));
}
/** Flushes buffered telemetry and stops the SDK. Safe to call more than once. */
async function shutdown() {
    restore.splice(0).forEach((fn) => fn());
    const s = sdk;
    sdk = undefined;
    if (s)
        await s.shutdown().catch((e) => api_1.diag.warn(`[resolvex] shutdown: ${e}`));
}
/**
 * Tells Resolve-X that `version` of this service was just deployed, so incident
 * analysis can connect a degradation to a release.
 */
async function markDeployment(opts = {}) {
    if (!current)
        throw new Error("[resolvex] call init() first");
    const body = {
        service: opts.service ?? current.serviceName,
        version: opts.version ?? current.version,
        environment: opts.environment ?? current.environment,
    };
    if (!body.version)
        throw new Error("[resolvex] markDeployment needs a version");
    const res = await fetch(`${current.apiUrl}/api/v1/webhooks/deployments`, {
        method: "POST",
        headers: { "content-type": "application/json", "x-resolvex-key": current.apiKey },
        body: JSON.stringify(body),
    });
    if (!res.ok)
        throw new Error(`[resolvex] deployment marker rejected: HTTP ${res.status}`);
}
const severity = {
    debug: [api_logs_1.SeverityNumber.DEBUG, "DEBUG"],
    info: [api_logs_1.SeverityNumber.INFO, "INFO"],
    warn: [api_logs_1.SeverityNumber.WARN, "WARN"],
    error: [api_logs_1.SeverityNumber.ERROR, "ERROR"],
};
function emit(level, body, attributes) {
    const [num, text] = severity[level];
    // Passing the active context is what links the record to the current trace.
    api_logs_1.logs.getLogger("resolvex").emit({ severityNumber: num, severityText: text, body, attributes, context: api_1.context.active() });
}
/** Structured logger whose records are linked to the active trace. */
exports.logger = {
    debug: (msg, attrs) => emit("debug", msg, attrs),
    info: (msg, attrs) => emit("info", msg, attrs),
    warn: (msg, attrs) => emit("warn", msg, attrs),
    error: (msg, attrs) => emit("error", msg, attrs),
};
/** Runs `fn` inside a named span; records exceptions and sets error status. */
async function withSpan(name, fn, attributes) {
    const tracer = api_1.trace.getTracer("resolvex");
    return tracer.startActiveSpan(name, { attributes }, async (span) => {
        try {
            return await fn();
        }
        catch (e) {
            span.recordException(e);
            span.setStatus({ code: 2, message: e instanceof Error ? e.message : String(e) });
            throw e;
        }
        finally {
            span.end();
        }
    });
}
function captureConsole() {
    for (const [method, level] of [["log", "info"], ["info", "info"], ["warn", "warn"], ["error", "error"], ["debug", "debug"]]) {
        const original = console[method].bind(console);
        console[method] = (...args) => {
            original(...args);
            try {
                emit(level, args.map((a) => (typeof a === "string" ? a : JSON.stringify(a))).join(" "));
            }
            catch {
                /* never let telemetry break the app */
            }
        };
        restore.push(() => { console[method] = original; });
    }
}
