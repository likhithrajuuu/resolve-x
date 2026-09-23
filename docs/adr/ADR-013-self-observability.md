# ADR-013: Resolve-X Must Be Observable

## Status

Accepted

## Context

A monitoring and incident platform that cannot monitor itself creates an operational blind spot.

## Decision

Every Resolve-X service will expose:

* Structured logs
* Metrics
* Distributed traces
* Health endpoints

Resolve-X will use its own observability architecture wherever practical.

## Goal

```text
Resolve-X
    │
    └── monitored by Resolve-X
```

This creates a useful dogfooding environment and ensures the platform exercises the same telemetry pipeline it provides to customers.
