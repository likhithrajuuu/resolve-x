# Resolve-X Services

One document per deployable component.

Boundaries are defined in [ADR-006](../../adr/ADR-006-service-boundaries.md). The overall picture is in the [container diagram](../container-diagram.md).

| Service                                        | Summary                                                       |
| ---------------------------------------------- | ------------------------------------------------------------- |
| [Resolve-X Collector](collector.md)            | Runs in the customer environment; buffers and forwards OTLP.  |
| [API Gateway](api-gateway.md)                  | External entry point for the dashboard and integrations.      |
| [Ingestion Service](ingestion-service.md)      | Accepts and validates telemetry, publishes to Kafka.          |
| [Service Registry](service-registry.md)        | Discovered services and the dependency graph.                 |
| [Incident Service](incident-service.md)        | Incident lifecycle and timeline.                              |
| [Correlation Service](correlation-service.md)  | Builds evidence sets for incidents.                           |
| [Analysis Service](analysis-service.md)        | AI-assisted root-cause analysis.                              |

---

## Document Structure

Each service document uses the same sections:

```text
Purpose
Responsibilities
Does not own
API
Events
Data
Failure behaviour
Observability
```

APIs and schemas in these documents are **initial designs**. They will be refined, and recorded in code as OpenAPI specifications and event schemas, as each milestone is implemented.

---

## Conventions for All Services

* Spring Boot, one deployable per service.
* REST APIs versioned under `/api/v1`.
* Every request and event carries tenant context (ADR-010).
* Health endpoints: `/actuator/health/liveness` and `/actuator/health/readiness`.
* OpenTelemetry instrumentation for logs, metrics and traces (ADR-013).
* Configuration through environment variables; secrets never committed.
* Database migrations owned by the service that owns the tables.
