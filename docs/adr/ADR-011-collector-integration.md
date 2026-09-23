# ADR-011: Agent/Collector-Based Customer Integration

## Status

Accepted

## Context

Resolve-X should be plug-and-play and should minimize application changes.

Sending every application's telemetry directly to Resolve-X would also make customer applications dependent on the Resolve-X control plane.

## Decision

Resolve-X will support an agent/collector architecture.

```text
Customer Applications
        ↓
OTel SDK / Auto Instrumentation
        ↓
Resolve-X Collector
        ↓
Resolve-X Cloud
```

## Benefits

* Local buffering
* Reduced application coupling
* Centralized credential management
* Metadata enrichment
* Filtering
* Network resilience
* Kubernetes integration

## Consequence

Resolve-X must maintain and document collector deployment mechanisms.
