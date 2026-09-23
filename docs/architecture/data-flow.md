# Resolve-X — Data Flow

This document describes how data moves through Resolve-X: the flows, the Kafka topics that carry them, and the rules every producer and consumer follows.

High-level flows are also summarised in [system-architecture.md](system-architecture.md#6-data-flow).

Items marked **(proposed)** are initial designs that still need an ADR or validation during implementation.

---

## 1. End-to-End Flow

```text
Application
   │ OTLP
   ▼
Resolve-X Collector ── adds k8s metadata, buffers, batches
   │ OTLP + API key
   ▼
Ingestion Service ── authenticate, resolve tenant, validate
   │
   ▼
telemetry.raw ───────────────────────────────┐
   │                                         │
   ▼                                         ▼
Telemetry Processors                   Discovery Engine
   │                                         │
   ├── telemetry.logs                        ▼
   ├── telemetry.metrics               Service Registry ── service.discovered
   ├── telemetry.traces                      ▲             service.updated
   │                                         │
   ├── dependency.observed ──────────────────┘
   │
   └── write to telemetry stores

Alert / anomaly / manual
   │
   ▼
Incident Service ── incident.created ──▶ correlation.requested
                                                │
                                                ▼
                                      Correlation Service
                                                │
                                                ▼
                                      correlation.completed
                                          │           │
                                          ▼           ▼
                               Incident Service   analysis.requested
                               (timeline, evidence)    │
                                                       ▼
                                               Analysis Service ──▶ LLM
                                                       │
                                                       ▼
                                               analysis.completed
                                                       │
                                                       ▼
                                               Incident Service
```

---

## 2. Kafka Topics

| Topic                    | Producer              | Consumers                               | Key (proposed)            |
| ------------------------ | --------------------- | --------------------------------------- | ------------------------- |
| `telemetry.raw`          | Ingestion Service     | Telemetry Processors, Discovery Engine  | `tenantId:serviceName`    |
| `telemetry.logs`         | Telemetry Processors  | Log store writer, anomaly detection     | `tenantId:serviceName`    |
| `telemetry.metrics`      | Telemetry Processors  | Metric store writer, anomaly detection  | `tenantId:serviceName`    |
| `telemetry.traces`       | Telemetry Processors  | Trace store writer, dependency extraction | `tenantId:traceId`      |
| `dependency.observed`    | Telemetry Processors  | Service Registry                        | `tenantId:source:target`  |
| `deployment.recorded`    | Ingestion / Webhooks  | Service Registry, Correlation Service   | `tenantId:serviceId`      |
| `service.discovered`     | Service Registry      | Incident, Correlation, Dashboard        | `tenantId:serviceId`      |
| `service.updated`        | Service Registry      | Incident, Correlation, Dashboard        | `tenantId:serviceId`      |
| `incident.created`       | Incident Service      | Correlation, notifications              | `tenantId:incidentId`     |
| `incident.updated`       | Incident Service      | Dashboard, notifications                | `tenantId:incidentId`     |
| `correlation.requested`  | Incident Service      | Correlation Service                     | `tenantId:incidentId`     |
| `correlation.completed`  | Correlation Service   | Incident Service, Analysis Service      | `tenantId:incidentId`     |
| `analysis.requested`     | Incident Service      | Analysis Service                        | `tenantId:incidentId`     |
| `analysis.completed`     | Analysis Service      | Incident Service                        | `tenantId:incidentId`     |

`dependency.observed`, `deployment.recorded`, `correlation.requested` and `correlation.completed` extend the topics listed in ADR-002 and the system architecture.

### Keying Rationale

Kafka guarantees ordering only within a partition.

* Incident topics are keyed by incident so that all events for one incident are processed in order.
* Telemetry topics are keyed by service so that a service's telemetry stays together.
* Trace topics are keyed by trace so that spans of one trace reach the same consumer for dependency extraction.

Every key starts with the tenant ID so that tenant context is always visible at the transport level (ADR-010).

---

## 3. Event Envelope (proposed)

All domain events share a common envelope.

```json
{
  "eventId": "01J8Z3K8Q6V3YV3X4M5N6P7Q8R",
  "eventType": "incident.created",
  "eventVersion": 1,
  "occurredAt": "2026-09-23T10:32:15.123Z",
  "producer": "incident-service",
  "tenantId": "tnt_123",
  "projectId": "prj_456",
  "environmentId": "env_prod",
  "traceparent": "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01",
  "payload": {}
}
```

| Field           | Purpose                                                              |
| --------------- | -------------------------------------------------------------------- |
| `eventId`       | Unique ID used for idempotent consumption (NFR-004).                 |
| `eventType`     | Matches the topic name.                                              |
| `eventVersion`  | Schema version of the payload.                                       |
| `occurredAt`    | When the fact happened, not when it was published.                   |
| `tenantId`      | Mandatory. Events without it are rejected.                           |
| `traceparent`   | W3C trace context so Resolve-X's own processing is traceable (ADR-013). |

Serialization format and schema registry usage are an open decision (see the [ADR index](../adr/README.md#open-decisions)).

---

## 4. Delivery Rules

### At-least-once delivery

Consumers must assume duplicates.

Each consumer records processed `eventId`s, or makes its writes naturally idempotent, for example with upserts keyed by business identity.

### Transactional outbox (proposed)

Services that change PostgreSQL state and publish an event write both in one database transaction to an outbox table.

A relay publishes outbox rows to Kafka.

This prevents a state change being saved without its event, or an event being published for a change that was rolled back.

### Retries and dead letters

```text
consume ──▶ fail ──▶ retry with backoff ──▶ fail ──▶ <topic>.dlq
```

Messages in a dead-letter topic are visible in metrics and can be replayed after a fix.

### Schema evolution

* Adding optional fields is allowed without a version change.
* Removing or changing the meaning of a field requires a new `eventVersion`.
* Consumers ignore unknown fields.

---

## 5. Telemetry Flow Detail

```text
1. Application exports OTLP to the local collector.
2. Collector adds k8s.namespace.name, k8s.deployment.name, k8s.pod.name, k8s.node.name.
3. Collector batches, compresses and sends to the Ingestion Service.
4. Ingestion Service:
   a. authenticates the API key
   b. resolves tenant, project and environment
   c. validates payload size and required attributes
   d. publishes to telemetry.raw
   e. returns success to the collector
5. Telemetry Processors split by signal type, normalise attributes
   and write to telemetry stores.
6. Discovery Engine extracts resource attributes and updates the Service Registry.
```

Step 4e happens after the event is durably written to Kafka, not after processing. This keeps ingestion fast and independent of downstream work (NFR-003, NFR-005).

### Backpressure

```text
Downstream slow
      │
      ▼
Kafka absorbs the backlog (consumer lag grows)
      │
      ▼
Ingestion keeps accepting while Kafka is healthy
      │
      ▼
If Kafka is unavailable, Ingestion returns a retryable error
      │
      ▼
Collector buffers locally and retries
```

The application is never blocked (ADR-011).

---

## 6. Dependency Discovery Detail

Dependencies are derived from spans.

| Span evidence                                       | Dependency type   |
| --------------------------------------------------- | ----------------- |
| `CLIENT` span matched to a `SERVER` span in another service | `service`  |
| `db.system` attribute (for example `postgresql`)    | `database`        |
| `db.system = redis`                                 | `cache`           |
| `messaging.system` attribute (for example `kafka`)  | `message_broker`  |
| `CLIENT` span with `server.address` not owned by a known service | `external_api` |

Observations are aggregated over short windows before publishing to `dependency.observed`, to avoid one event per span.

---

## 7. Incident and Analysis Flow Detail

```text
1. Trigger arrives (alert, threshold, anomaly, manual).
2. Incident Service computes a deduplication key
   (tenant + service + signal type), checked in Redis.
3. Incident persisted; incident.created published (via outbox).
4. correlation.requested published with the investigation window:
      [start − lookback, now]
5. Correlation Service gathers:
      - metric changes for the affected service and its dependencies
      - error logs in the window
      - slow or failed traces
      - deployments in the window
      - dependency health
6. Evidence set persisted; correlation.completed published.
7. Incident Service attaches evidence and timeline entries.
8. analysis.requested published.
9. Analysis Service builds context, calls the LLM, validates output.
10. analysis.completed published; Incident Service stores a reference.
```

If steps 5–10 fail or are slow, the incident created in step 3 remains visible and usable (NFR-009).

---

## 8. Data Retention (proposed)

| Data                         | Store                     | Initial retention            |
| ---------------------------- | ------------------------- | ---------------------------- |
| Kafka telemetry topics       | Kafka                     | 24 hours                     |
| Kafka domain event topics    | Kafka                     | 7 days                       |
| Raw logs and traces          | Loki / Tempo / ClickHouse | 7–14 days                    |
| Metrics                      | Prometheus / ClickHouse   | 30 days                      |
| Services, dependencies       | PostgreSQL                | Until deleted by tenant      |
| Incidents, evidence, analyses| PostgreSQL                | Until deleted by tenant      |

Evidence sets keep the telemetry excerpts they reference, so incidents remain explainable after raw telemetry expires.

Retention will become configurable per tenant.

---

## 9. Tenant Isolation in the Data Flow

Tenant context is established once, at the ingestion or API boundary, from the credential.

It is never taken from untrusted payload content.

From there it is carried in:

* Kafka event envelopes and keys
* database rows (`tenant_id` on every tenant-owned table)
* Redis keys (`tenant:{tenantId}:...`)
* telemetry store labels
* LLM analysis requests, which only ever contain one tenant's evidence
