# Resolve-X

> AI-powered incident management and root-cause analysis for distributed applications.

Resolve-X is an observability and incident intelligence platform designed to automatically discover services, collect telemetry, correlate logs, metrics, traces, deployments and dependencies, and assist engineers in identifying the probable root cause of production incidents.

The goal is simple:

**When something breaks, Resolve-X should help engineers understand what happened, why it happened, and what to do next.**

---

## Problem

Modern applications are increasingly distributed across multiple services, databases, message brokers, caches and external APIs.

When an incident occurs, engineers often need to manually:

1. Identify the affected service.
2. Search application logs.
3. Inspect metrics.
4. Follow distributed traces.
5. Investigate service dependencies.
6. Check recent deployments and configuration changes.
7. Correlate events occurring around the same time.
8. Determine the probable root cause.
9. Decide what remediation should be performed.

This investigation can take significant time, especially when the system contains dozens or hundreds of services.

Resolve-X aims to reduce this investigation time by automatically collecting and correlating operational evidence.

---

## Vision

Resolve-X aims to become an AI-assisted incident investigation platform that can understand a distributed system as it operates.

The platform should automatically answer questions such as:

* What services are currently running?
* Which services depend on each other?
* Which service is affected by an incident?
* What changed before the incident?
* Which logs, metrics and traces are related?
* Where did the failure originate?
* Has this failure happened before?
* What is the probable root cause?
* What actions could resolve the incident?

---

## Core Capabilities

### 1. Automatic Service Discovery

Resolve-X discovers services from runtime telemetry and infrastructure metadata rather than requiring engineers to manually register every service.

The initial implementation will use:

* OpenTelemetry service metadata
* Kubernetes metadata
* Runtime dependency observations

---

### 2. Multi-Language Support

Resolve-X is designed to support applications written in different languages and frameworks.

Initial targets:

* Java / Spring Boot
* Python / FastAPI
* Node.js
* Go

OpenTelemetry and OTLP will provide the common telemetry contract between applications and Resolve-X.

---

### 3. Telemetry Collection

Resolve-X will work with:

* Logs
* Metrics
* Distributed traces
* Deployment events
* Infrastructure metadata
* Application events

---

### 4. Service Dependency Mapping

Resolve-X will construct a continuously updated dependency graph.

Example:

```text
API Gateway
    │
    ├── Order Service
    │       │
    │       ├── PostgreSQL
    │       └── Redis
    │
    └── Payment Service
            │
            ├── PostgreSQL
            └── External Payment API
```

---

### 5. Incident Management

Resolve-X will create and manage incidents based on alerts and detected anomalies.

Each incident will contain:

* Severity
* Affected services
* Start time
* Impact
* Related telemetry
* Recent changes
* Dependency information
* Incident timeline
* Probable root cause
* Recommended actions

---

### 6. Automated Correlation

Resolve-X will correlate multiple signals around an incident.

For example:

```text
10:32:00  Deployment
10:32:05  CPU increases
10:32:08  Database latency increases
10:32:10  HTTP 5xx increases
10:32:14  Payment timeout
10:32:15  Incident created
```

Instead of presenting these as unrelated events, Resolve-X will attempt to identify their relationship.

---

### 7. AI-Assisted Investigation

AI will not replace the telemetry and correlation pipeline.

Instead:

```text
Raw Telemetry
      ↓
Evidence Collection
      ↓
Correlation
      ↓
Incident Context
      ↓
AI Analysis
      ↓
Probable Root Cause
      ↓
Recommended Actions
```

The AI layer will reason over structured evidence collected by Resolve-X.

---

## High-Level Architecture

```text
Customer Applications
        │
        │ OpenTelemetry
        ▼
Resolve-X Agent / Collector
        │
        │ OTLP
        ▼
Ingestion Layer
        │
        ▼
Event Streaming
        │
        ▼
Telemetry Processing
        │
        ├── Logs
        ├── Metrics
        └── Traces
        │
        ▼
Correlation Engine
        │
        ├── Service Registry
        ├── Dependency Graph
        └── Incident Timeline
        │
        ▼
AI Analysis
        │
        ▼
Incident Management
        │
        ▼
Resolve-X Dashboard
```

---

## Design Principles

Resolve-X will follow these principles:

### Open standards first

Prefer OpenTelemetry and other open standards instead of creating proprietary telemetry protocols where possible.

### Automation first

The platform should minimize manual configuration and registration.

### Evidence before AI

AI should analyze collected evidence rather than operate on incomplete or unstructured assumptions.

### Human-in-the-loop

AI-generated diagnoses and remediation suggestions should remain explainable and reviewable by engineers.

### Observable by design

Resolve-X itself must be observable.

### Event-driven where appropriate

Asynchronous processing should be used for telemetry and incident workflows where it improves scalability and resilience.

### Explicit architectural decisions

Significant architectural decisions will be documented using Architecture Decision Records (ADRs).

---

## Initial Technology Direction

| Area                         | Initial Technology     |
| ---------------------------- | ---------------------- |
| Backend                      | Java / Spring Boot     |
| API                          | REST                   |
| Internal RPC                 | gRPC where justified   |
| Event streaming              | Apache Kafka           |
| Transactional database       | PostgreSQL             |
| Cache / transient state      | Redis                  |
| Telemetry standard           | OpenTelemetry          |
| Metrics                      | Prometheus             |
| Logs                         | Loki                   |
| Traces                       | Tempo                  |
| Analytical telemetry storage | ClickHouse             |
| Containerization             | Docker                 |
| Orchestration                | Kubernetes             |
| AI                           | Pluggable LLM provider |
| Frontend                     | To be determined       |

Technology choices are subject to change and must be supported by documented architectural decisions.

---

## Project Status

**Current phase:** Architecture and product definition.

The implementation will initially focus on a minimal end-to-end telemetry and service-discovery pipeline before expanding into automated incident correlation and AI-assisted investigation.

---

## Documentation

```text
docs/
├── product/
│   ├── vision.md
│   ├── problem-statement.md
│   ├── requirements.md
│   ├── use-cases.md
│   └── roadmap.md
│
├── architecture/
│   ├── system-architecture.md
│   ├── system-context.md
│   ├── container-diagram.md
│   ├── data-flow.md
│   └── services/
│       ├── collector.md
│       ├── api-gateway.md
│       ├── ingestion-service.md
│       ├── service-registry.md
│       ├── incident-service.md
│       ├── correlation-service.md
│       └── analysis-service.md
│
└── adr/
    ├── README.md
    ├── template.md
    ├── ADR-001-opentelemetry.md
    ├── ADR-002-kafka.md
    ├── ADR-003-postgresql.md
    ├── ADR-004-redis.md
    ├── ADR-005-service-discovery.md
    ├── ADR-006-service-boundaries.md
    ├── ADR-007-rest-vs-rpc.md
    ├── ADR-008-ai-analysis.md
    ├── ADR-009-human-approval-remediation.md
    ├── ADR-010-multi-tenancy.md
    ├── ADR-011-collector-integration.md
    ├── ADR-012-telemetry-analytics-store.md
    └── ADR-013-self-observability.md
```

Suggested reading order:

1. [Vision](docs/product/vision.md) and [problem statement](docs/product/problem-statement.md)
2. [Requirements](docs/product/requirements.md) and [use cases](docs/product/use-cases.md)
3. [System context](docs/architecture/system-context.md) and [container diagram](docs/architecture/container-diagram.md)
4. [Data flow](docs/architecture/data-flow.md) and [service documents](docs/architecture/services/README.md)
5. [Architecture decision records](docs/adr/README.md)
6. [Roadmap](docs/product/roadmap.md)

---

## Long-Term Goal

Resolve-X should move incident management from:

```text
Something broke
      ↓
Engineer searches everywhere
      ↓
Engineer correlates evidence manually
      ↓
Engineer guesses root cause
      ↓
Engineer decides what to do
```

toward:

```text
Something broke
      ↓
Resolve-X collects evidence
      ↓
Resolve-X correlates signals
      ↓
Resolve-X explains the incident
      ↓
Resolve-X identifies probable causes
      ↓
Resolve-X recommends actions
      ↓
Engineer reviews and acts
```
