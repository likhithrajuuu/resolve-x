import { type AnyValueMap } from "@opentelemetry/api-logs";
import { type Options } from "./config";
export type { Options } from "./config";
/**
 * Starts tracing, metrics and logs and sends them to Resolve-X.
 * Call once, as early as possible (before importing the libraries you want
 * instrumented), or use `node -r @resolve-x/sdk/register app.js`.
 */
export declare function init(options?: Options): void;
/** Flushes buffered telemetry and stops the SDK. Safe to call more than once. */
export declare function shutdown(): Promise<void>;
/**
 * Tells Resolve-X that `version` of this service was just deployed, so incident
 * analysis can connect a degradation to a release.
 */
export declare function markDeployment(opts?: {
    service?: string;
    version?: string;
    environment?: string;
}): Promise<void>;
/** Structured logger whose records are linked to the active trace. */
export declare const logger: {
    debug: (msg: string, attrs?: AnyValueMap) => void;
    info: (msg: string, attrs?: AnyValueMap) => void;
    warn: (msg: string, attrs?: AnyValueMap) => void;
    error: (msg: string, attrs?: AnyValueMap) => void;
};
/** Runs `fn` inside a named span; records exceptions and sets error status. */
export declare function withSpan<T>(name: string, fn: () => Promise<T> | T, attributes?: Record<string, string | number | boolean>): Promise<T>;
