# Ingestion Service

## Purpose

Accept telemetry from collectors quickly and safely, and hand it to the asynchronous pipeline.

---

## Responsibilities

* Receive OTLP over gRPC and HTTP.
* Authenticate the API key.
* Resolve tenant, project and environment from the key.
* Validate payload size and required attributes.
* Add ingestion metadata (received time, tenant context).
* Publish to `telemetry.raw`.

---

## Does Not Own

* Incident lifecycle.
* AI analysis.
* Long-term telemetry storage or analytics.
* Service identity decisions.

It must not perform expensive processing on the request path.

---

## API

| Endpoint                  | Protocol   | Notes                            |
| ------------------------- | ---------- | -------------------------------- |
| `:4317`                   | OTLP/gRPC  | Traces, metrics, logs            |
| `POST /v1/traces`         | OTLP/HTTP  |                                  |
| `POST /v1/metrics`        | OTLP/HTTP  |                                  |
| `POST /v1/logs`           | OTLP/HTTP  |                                  |

Paths follow the OTLP specification so standard exporters work unchanged.

### Responses

| Situation                          | Response                                   |
| ---------------------------------- | ------------------------------------------ |
| Accepted and written to Kafka      | Success                                    |
| Invalid or revoked key             | Unauthenticated, not retryable             |
| Payload too large / invalid        | Bad request, not retryable                 |
| Tenant rate limit exceeded         | Retryable with backoff                     |
| Kafka unavailable                  | Retryable; collector buffers               |

---

## Events

| Direction | Topic           |
| --------- | --------------- |
| Produces  | `telemetry.raw` |

See [data-flow.md](../data-flow.md) for keys and the event envelope.

---

## Data

No owned tables.

API key lookups are cached in Redis with a short TTL to avoid a database call per request.

---

## Downstream: Telemetry Processors

Separate consumers of `telemetry.raw`, deployed independently from the Ingestion Service so that heavy processing does not slow acceptance.

Responsibilities:

* Split by signal type into `telemetry.logs`, `telemetry.metrics`, `telemetry.traces`.
* Normalise attributes to a common model.
* Write to telemetry stores.
* Extract dependency observations from spans and publish `dependency.observed`.

---

## Failure Behaviour

* Stateless; any instance can serve any request.
* If Kafka is down, return a retryable error rather than accepting data that could be lost.
* A slow downstream consumer never affects ingestion latency.

---

## Observability

* Accepted and rejected items per signal type and tenant.
* Request latency (target p95 < 200 ms, NFR-005).
* Kafka publish latency and failures.
* Payload sizes.
