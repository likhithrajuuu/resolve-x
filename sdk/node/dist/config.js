"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
exports.resolve = resolve;
const trimSlash = (u) => u.replace(/\/+$/, "");
const truthy = (v) => v === "1" || v === "true";
function resolve(o = {}, env = process.env) {
    const apiKey = o.apiKey ?? env.RESOLVEX_API_KEY;
    if (!apiKey)
        throw new Error("[resolvex] apiKey is required (pass { apiKey } or set RESOLVEX_API_KEY)");
    const serviceName = o.serviceName ?? env.RESOLVEX_SERVICE ?? env.OTEL_SERVICE_NAME;
    if (!serviceName)
        throw new Error("[resolvex] serviceName is required (pass { serviceName } or set RESOLVEX_SERVICE)");
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
