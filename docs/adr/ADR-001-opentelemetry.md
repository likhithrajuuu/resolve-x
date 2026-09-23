# ADR-001: Use OpenTelemetry for Telemetry

## Status

Accepted

## Context

Resolve-X must support applications written in multiple programming languages and frameworks.

A proprietary telemetry SDK would require Resolve-X to maintain separate instrumentation systems for each ecosystem.

## Decision

Resolve-X will use OpenTelemetry and OTLP as its primary telemetry standard.

## Alternatives

* Custom Resolve-X telemetry protocol
* Prometheus-only architecture
* Vendor-specific agents
* OpenTelemetry

## Rationale

OpenTelemetry provides a common model for logs, metrics and traces and allows Resolve-X to support multiple languages without creating a proprietary telemetry protocol.

## Consequences

### Positive

* Multi-language support
* Vendor neutrality
* Standard instrumentation
* Existing ecosystem
* Reduced custom instrumentation

### Negative

* Additional collector infrastructure
* OpenTelemetry concepts must be understood
* Some framework instrumentation may differ
