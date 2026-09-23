# Resolve-X — Roadmap

This roadmap describes the planned order of delivery.

It is organised by milestones rather than dates. Each milestone should leave the platform in a working, demonstrable state.

The first target is the success definition from the [vision](vision.md#7-definition-of-success):

> A developer can connect a real application to Resolve-X, and Resolve-X can automatically discover the application, receive telemetry, identify its dependencies and produce useful incident context without requiring manual telemetry management.

---

## Overview

```text
M0  Foundations
 ↓
M1  Telemetry Ingestion
 ↓
M2  Service Discovery
 ↓
M3  Dependency Mapping
 ↓
M4  Incident Management
 ↓
M5  Correlation
 ↓
M6  AI-Assisted Analysis
 ↓
M7  Dashboard and Integrations
 ↓
M8  Production Readiness
```

---

## M0 — Foundations

**Goal:** a repository and local environment that every later milestone builds on.

### Scope

* Multi-module Java / Spring Boot project structure.
* Docker Compose environment with Kafka, PostgreSQL and Redis.
* Shared conventions for logging, configuration, error handling and tenant context.
* Database migration tooling.
* CI pipeline for build and tests.
* Resolve-X services instrumented with OpenTelemetry from the start (ADR-013).

### Exit Criteria

* `docker compose up` starts the local infrastructure.
* An empty service builds, starts, exposes health endpoints and emits telemetry.

---

## M1 — Telemetry Ingestion

**Goal:** accept telemetry from a real application.

**Requirements:** FR-006, FR-007, FR-008, FR-009, NFR-003

### Scope

* Ingestion Service accepting OTLP over gRPC and HTTP.
* API key authentication and tenant resolution.
* Validation and publishing to Kafka topics.
* Resolve-X Collector configuration based on the OpenTelemetry Collector.
* A sample multi-service demo application.

### Exit Criteria

* The demo application sends logs, metrics and traces through the collector.
* Telemetry appears on the expected Kafka topics with tenant context.
* Invalid credentials are rejected.

---

## M2 — Service Discovery

**Goal:** services appear without manual registration.

**Requirements:** FR-004, FR-005

### Scope

* Service Registry with PostgreSQL persistence.
* Discovery from OpenTelemetry resource attributes.
* Kubernetes metadata enrichment through the collector.
* `service.discovered` and `service.updated` events.
* REST API for listing services.

### Exit Criteria

* Every demo service is listed with name, language, version, environment and first/last seen.
* A redeployed service shows its new version.

---

## M3 — Dependency Mapping

**Goal:** know what each service talks to.

**Requirements:** FR-010, FR-011

### Scope

* Dependency extraction from client/server and database spans.
* Detection of databases, caches, message brokers and external APIs.
* Aggregated request count, error count and latency per edge.
* REST API for the dependency graph.

### Exit Criteria

* The demo application's dependency graph matches its real architecture.

---

## M4 — Incident Management

**Goal:** incidents are created, tracked and resolved.

**Requirements:** FR-012, FR-013, NFR-004, NFR-009

### Scope

* Incident Service with the lifecycle defined in the [architecture](../architecture/system-architecture.md).
* Incident creation from threshold rules and from an alert webhook.
* Deduplication.
* Incident timeline.
* Manual incident creation.

### Exit Criteria

* Injecting a failure into the demo application creates exactly one incident.
* The incident can be moved through its lifecycle to CLOSED.

---

## M5 — Correlation

**Goal:** incidents contain the evidence an engineer would otherwise search for.

**Requirements:** FR-014, FR-015

### Scope

* Correlation Service.
* Investigation window selection.
* Time, service, trace, dependency and deployment correlation.
* Deployment event ingestion.
* Evidence set persisted and linked to the incident.

### Exit Criteria

* For the injected failure, the incident shows the related logs, metric changes, traces, affected dependency and recent deployment.

---

## M6 — AI-Assisted Analysis

**Goal:** a probable cause, explained with evidence.

**Requirements:** FR-016, FR-017, FR-018, FR-019, NFR-010

### Scope

* Analysis Service with a pluggable LLM provider.
* Incident context construction from the evidence set (ADR-008).
* Structured output validation.
* Evidence references in every conclusion.
* Recommendations with human approval (ADR-009).
* Evaluation suite of known failure scenarios.

### Exit Criteria

* The analysis for each scenario in the evaluation suite cites evidence and identifies the injected cause.
* Incidents remain fully usable when the Analysis Service is stopped.

---

## M7 — Dashboard and Integrations

**Goal:** engineers can use Resolve-X without calling APIs directly.

**Requirements:** FR-001, FR-002, FR-003, FR-020

### Scope

* Frontend technology decision recorded as an ADR.
* Dashboard for services, dependency graph and incidents.
* Tenant, project, environment and API key management.
* Notification channels: Slack and webhooks first.

### Exit Criteria

* The full onboarding and investigation flow (UC-001 to UC-008) is possible through the dashboard.

---

## M8 — Production Readiness

**Goal:** Resolve-X can be operated reliably.

**Requirements:** NFR-001, NFR-002, NFR-005, NFR-006, NFR-007, NFR-008

### Scope

* Kubernetes deployment manifests or Helm charts.
* Load tests against the ingestion path.
* Tenant isolation tests.
* Audit logging.
* Secret management.
* Resolve-X monitoring itself (UC-011).
* Analytical telemetry storage (ClickHouse) if volume requires it (ADR-012).

### Exit Criteria

* Latency targets in NFR-005 are measured and met under the agreed test load.
* Automated tests prove that tenants cannot access each other's data.

---

## Later

Items intentionally deferred beyond the first release:

* Historical incident similarity ("has this happened before?").
* Learned anomaly detection.
* Approved automated remediation with rollback.
* Post-incident report generation.
* VM and bare-metal discovery.
* Additional notification channels such as Microsoft Teams, PagerDuty and email.
