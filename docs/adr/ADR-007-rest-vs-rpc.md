# ADR-007: REST vs gRPC vs Kafka

## Status

Accepted

## Decision

### REST

Use for:

```text
External API
Dashboard
Administrative operations
Incident queries
Service queries
```

### Kafka

Use for:

```text
Telemetry events
Domain events
Asynchronous processing
Incident notifications
Analysis workflows
```

### gRPC

Use selectively for internal synchronous calls where strong contracts and low-overhead RPC provide a meaningful benefit.

## Example

```text
Dashboard
    │
    │ REST
    ▼
API Gateway
    │
    ▼
Incident Service

Telemetry
    │
    ▼
Kafka
    │
    ├── Correlation
    ├── Incident
    └── Analysis
```

## Rationale

Different communication patterns have different requirements.

The architecture should not force one protocol onto every interaction.
