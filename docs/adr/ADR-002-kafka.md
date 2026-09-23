# ADR-002: Use Kafka for Asynchronous Event Processing

## Status

Accepted

## Context

Telemetry volume can be significantly larger than incident volume.

Processing telemetry synchronously through HTTP services would tightly couple ingestion to downstream processing.

## Decision

Kafka will be used as the primary asynchronous event backbone.

## Events

Examples:

```text
telemetry.received
service.discovered
service.updated
incident.created
incident.updated
analysis.requested
analysis.completed
```

## Alternatives

* Direct REST calls
* RabbitMQ
* Kafka
* Database polling

## Rationale

Kafka provides durable event streaming, consumer groups and independent scaling of processing components.

## Consequences

### Positive

* Decoupled services
* Replay capability
* Independent consumers
* Horizontal scalability
* Backpressure handling

### Negative

* Operational complexity
* Event schema management
* Eventual consistency
* Duplicate delivery must be handled
