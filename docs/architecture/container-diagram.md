# Resolve-X — Container Diagram

This document describes the deployable units ("containers" in C4 terms) that make up Resolve-X and how they communicate.

It corresponds to level 2 of the C4 model.

Service boundaries follow [ADR-006](../adr/ADR-006-service-boundaries.md). Communication styles follow [ADR-007](../adr/ADR-007-rest-vs-rpc.md).

---

## 1. Container Diagram

```text
 Customer environment
┌───────────────────────────────────────────────┐
│  Applications ──OTLP──▶ Resolve-X Collector   │
│                         (OTel Collector +     │
│                          k8s metadata)        │
└───────────────────────────┬───────────────────┘
                            │ OTLP (gRPC / HTTP), TLS, API key
 Resolve-X                  ▼
┌───────────────────────────────────────────────────────────────────────────┐
│                                                                           │
│  ┌────────────┐   REST   ┌─────────────┐                                  │
│  │ Dashboard  │ ───────▶ │ API Gateway │ ─── REST ─────────┐              │
│  └────────────┘          └──────┬──────┘                   │              │
│                                 │ REST                     │              │
│                                 ▼                          ▼              │
│  ┌──────────────┐     ┌──────────────────┐       ┌──────────────────┐     │
│  │  Ingestion   │     │ Service Registry │       │ Incident Service │     │
│  │  Service     │     └───────┬──────────┘       └────────┬─────────┘     │
│  └──────┬───────┘             │  ▲                        │  ▲            │
│         │ produce             │  │ consume                │  │ consume    │
│         ▼                     ▼  │                        ▼  │            │
│  ╔═══════════════════════════════════════════════════════════════════╗    │
│  ║                              Kafka                                ║    │
│  ╚═══════════════════════════════════════════════════════════════════╝    │
│         │ consume                          ▲            │ consume  ▲      │
│         ▼                                  │ produce    ▼          │      │
│  ┌──────────────────┐             ┌────────┴─────────┐  ┌──────────┴───┐  │
│  │ Telemetry        │             │ Correlation      │  │ Analysis     │  │
│  │ Processors       │             │ Service          │  │ Service      │──┼──▶ LLM
│  │ (logs/metrics/   │             └──────────────────┘  └──────────────┘  │
│  │  traces)         │                                                     │
│  └──────────────────┘                                                     │
│                                                                           │
│  ┌──────────────┐   ┌────────────┐   ┌────────────────────────────────┐   │
│  │ PostgreSQL   │   │ Redis      │   │ Telemetry stores               │   │
│  │ (system of   │   │ (transient │   │ Prometheus / Loki / Tempo,     │   │
│  │  record)     │   │  state)    │   │ ClickHouse when justified      │   │
│  └──────────────┘   └────────────┘   └────────────────────────────────┘   │
└───────────────────────────────────────────────────────────────────────────┘
```

---

## 2. Containers

| Container            | Technology                     | Responsibility                                                    | Owns data                              | Details |
| -------------------- | ------------------------------ | ----------------------------------------------------------------- | -------------------------------------- | ------- |
| Resolve-X Collector  | OpenTelemetry Collector        | Buffer, enrich and forward customer telemetry.                    | None (local buffer only)               | [collector](services/collector.md) |
| API Gateway          | Spring Boot                    | External entry point: auth, routing, rate limiting.               | None                                   | [api-gateway](services/api-gateway.md) |
| Ingestion Service    | Spring Boot                    | Accept OTLP, authenticate, validate, publish to Kafka.            | None                                   | [ingestion-service](services/ingestion-service.md) |
| Telemetry Processors | Spring Boot                    | Normalise logs, metrics and traces; write to telemetry stores.    | Telemetry stores                       | [ingestion-service](services/ingestion-service.md#downstream-telemetry-processors) |
| Service Registry     | Spring Boot                    | Service identity, discovery and dependency graph.                 | `services`, `dependencies`             | [service-registry](services/service-registry.md) |
| Incident Service     | Spring Boot                    | Incident lifecycle, timeline, deduplication.                      | `incidents`, `incident_timeline`       | [incident-service](services/incident-service.md) |
| Correlation Service  | Spring Boot                    | Build evidence sets for incidents.                                | Evidence sets                          | [correlation-service](services/correlation-service.md) |
| Analysis Service     | Spring Boot                    | AI-assisted root-cause analysis and recommendations.              | `analyses`                             | [analysis-service](services/analysis-service.md) |
| Dashboard            | To be determined               | User interface.                                                   | None                                   | — |
| Kafka                | Apache Kafka                   | Asynchronous event backbone (ADR-002).                            | Event log                              | [data-flow](data-flow.md) |
| PostgreSQL           | PostgreSQL                     | Transactional system of record (ADR-003).                         | Domain data                            | — |
| Redis                | Redis                          | Transient, performance-sensitive state (ADR-004).                 | Caches, dedup keys, locks              | — |
| Telemetry stores     | Prometheus, Loki, Tempo, ClickHouse | Query-able logs, metrics, traces and analytics (ADR-012).     | Telemetry                              | — |

Tenant and user management is initially hosted by the API Gateway module set. It may become a separate service if its lifecycle or scaling differs (ADR-006 principle).

---

## 3. Data Ownership

Each service owns its tables. Other services do not read or write them directly.

```text
Service Registry   → services, service_versions, dependencies
Incident Service   → incidents, incident_timeline, remediation_actions
Correlation Service→ evidence_sets, evidence_items
Analysis Service   → analyses
API Gateway        → tenants, users, projects, environments, api_keys
```

In the initial deployment these may share one PostgreSQL instance using separate schemas.

Cross-service data is obtained through REST queries or Kafka events, never through shared tables.

---

## 4. Communication Summary

| From                 | To                  | Style  | Purpose                                     |
| -------------------- | ------------------- | ------ | ------------------------------------------- |
| Collector            | Ingestion Service   | OTLP   | Telemetry                                   |
| Dashboard            | API Gateway         | REST   | All user operations                         |
| API Gateway          | Internal services   | REST   | Queries and commands                        |
| Ingestion Service    | Kafka               | Events | Raw telemetry                               |
| Telemetry Processors | Kafka               | Events | Normalised telemetry, dependency observations |
| Service Registry     | Kafka               | Events | Service and dependency changes              |
| Incident Service     | Kafka               | Events | Incident lifecycle, correlation requests    |
| Correlation Service  | Service Registry    | REST   | Dependency lookups                          |
| Correlation Service  | Telemetry stores    | Query  | Evidence gathering                          |
| Correlation Service  | Kafka               | Events | Correlation results                         |
| Analysis Service     | LLM Provider        | HTTPS  | Analysis                                    |
| Analysis Service     | Kafka               | Events | Analysis results                            |

gRPC is not used initially. It will be introduced only where a synchronous internal call justifies it (ADR-007).

---

## 5. Scaling Characteristics

| Container            | Load driver               | Scaling approach                                 |
| -------------------- | ------------------------- | ------------------------------------------------ |
| Ingestion Service    | Telemetry volume          | Stateless, horizontal                            |
| Telemetry Processors | Telemetry volume          | Kafka consumer groups, partitions                |
| Service Registry     | Number of services        | Horizontal, cached reads                         |
| Incident Service     | Incident volume           | Horizontal; low volume relative to telemetry     |
| Correlation Service  | Incident volume × evidence | Horizontal consumers                            |
| Analysis Service     | Incident volume, LLM latency | Horizontal, rate-limited by provider           |

Telemetry and incident paths scale independently (NFR-002).
