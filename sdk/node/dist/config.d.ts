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
export declare function resolve(o?: Options, env?: NodeJS.ProcessEnv): Resolved;
