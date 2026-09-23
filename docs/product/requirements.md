# Resolve-X Product Requirements

## 1. Functional Requirements

### FR-001 — Tenant Management

Resolve-X shall support multiple organizations using the platform independently.

Each organization shall have isolated:

* Users
* Projects
* Environments
* Services
* Telemetry
* Incidents
* API credentials

---

### FR-002 — Project Management

An organization shall be able to create multiple projects.

Example:

```text
Acme Corporation
│
├── E-Commerce
├── Payments
└── Internal Platform
```

---

### FR-003 — Environment Management

Each project shall support environments such as:

```text
development
staging
production
```

Telemetry and incidents shall be associated with an environment.

---

### FR-004 — Service Discovery

Resolve-X shall automatically discover services from telemetry and infrastructure metadata.

The initial implementation shall use:

* OpenTelemetry resource attributes
* Kubernetes metadata
* Runtime dependency observations

A service should not require manual registration for basic discovery.

---

### FR-005 — Service Identity

Resolve-X shall maintain a canonical identity for each service.

A service record should contain information such as:

```text
service ID
tenant
project
environment
service name
language
framework
version
deployment
first seen
last seen
status
```

---

### FR-006 — Telemetry Ingestion

Resolve-X shall ingest telemetry using OpenTelemetry and OTLP.

The initial telemetry types are:

```text
Logs
Metrics
Traces
```

---

### FR-007 — Log Processing

Resolve-X shall receive and process structured application logs.

The system should support:

* Log severity
* Timestamp
* Service identity
* Environment
* Trace ID
* Span ID
* Exception information
* Structured attributes

---

### FR-008 — Metric Processing

Resolve-X shall process numerical time-series telemetry.

Examples:

```text
Request rate
Error rate
Latency
CPU
Memory
Database connections
Queue depth
Kafka consumer lag
```

---

### FR-009 — Trace Processing

Resolve-X shall process distributed traces.

The system shall be able to associate:

```text
Trace
    ↓
Span
    ↓
Service
    ↓
Dependency
```

---

### FR-010 — Dependency Discovery

Resolve-X shall automatically identify observed relationships between:

* Services
* Databases
* Caches
* Message brokers
* External APIs

Example:

```text
Order Service
     │
     ▼
Payment Service
     │
     ▼
PostgreSQL
```

---

### FR-011 — Service Dependency Graph

Resolve-X shall maintain a continuously updated dependency graph.

Each dependency should contain:

```text
source
target
dependency type
first observed
last observed
request count
error count
latency information
```

---

### FR-012 — Incident Creation

Resolve-X shall create incidents from:

* Alerts
* Threshold violations
* Anomaly detection
* External incident integrations
* Manually created incidents

---

### FR-013 — Incident Timeline

Every incident shall maintain a chronological timeline.

Example:

```text
10:31:58 Deployment
10:32:04 CPU increase
10:32:08 Database latency increase
10:32:11 HTTP errors increase
10:32:14 Timeout
10:32:15 Incident created
```

---

### FR-014 — Evidence Collection

For each incident, Resolve-X shall collect relevant:

* Logs
* Metrics
* Traces
* Deployments
* Service dependencies
* Infrastructure events

---

### FR-015 — Telemetry Correlation

Resolve-X shall correlate telemetry based on factors including:

* Time
* Service
* Trace
* Dependency
* Deployment
* Error type
* Metric behavior

---

### FR-016 — Root Cause Analysis

Resolve-X shall generate a probable root-cause analysis from correlated evidence.

The result shall distinguish between:

```text
Observed evidence
Inferred relationship
Probable cause
Uncertainty
```

---

### FR-017 — Evidence-Based AI Analysis

AI analysis shall receive a curated incident context rather than unrestricted access to all telemetry.

Example:

```text
Incident
    ↓
Relevant metrics
    ↓
Relevant logs
    ↓
Relevant traces
    ↓
Recent changes
    ↓
Dependencies
    ↓
AI analysis
```

---

### FR-018 — Remediation Recommendations

Resolve-X shall provide recommended investigation or remediation steps.

Example:

```text
1. Inspect database connection pool configuration.
2. Compare current deployment with previous version.
3. Inspect PostgreSQL connection saturation.
4. Consider rollback if deployment correlation is confirmed.
```

---

### FR-019 — Human Approval

Resolve-X shall distinguish between:

```text
Recommendation
Approved action
Executed action
```

AI-generated recommendations shall not automatically result in destructive actions unless explicitly configured and authorized.

---

### FR-020 — Notifications

Resolve-X shall support incident notifications through configurable channels.

Potential integrations:

```text
Email
Slack
Microsoft Teams
PagerDuty
Webhooks
```

These integrations are outside the initial MVP.

---

# 2. Non-Functional Requirements

## NFR-001 — Availability

Resolve-X control-plane services should be designed for horizontal scaling and failure isolation.

The initial target is:

```text
99.9% control-plane availability
```

This is an initial engineering target and will be validated through testing.

---

## NFR-002 — Scalability

Telemetry ingestion shall be independently scalable from incident management.

The architecture must allow:

```text
10 services
        ↓
100 services
        ↓
1,000+ services
```

without requiring architectural redesign.

---

## NFR-003 — Backpressure

Telemetry ingestion shall tolerate temporary downstream processing delays.

The ingestion pipeline shall support buffering and asynchronous processing.

---

## NFR-004 — Idempotency

Telemetry and domain events may be delivered more than once.

Consumers shall therefore be designed to safely handle duplicate events where required.

---

## NFR-005 — Latency

Initial targets:

```text
API p95 latency:             < 300 ms
Telemetry acceptance p95:    < 200 ms
Incident creation target:   < 5 seconds
```

These are engineering targets rather than guaranteed SLAs.

---

## NFR-006 — Security

Resolve-X shall provide:

* Authentication
* Authorization
* Tenant isolation
* API key management
* Encrypted communication
* Secret management
* Audit logging

---

## NFR-007 — Data Isolation

A tenant must never be able to access another tenant's telemetry or incidents.

Tenant context must be enforced at service and persistence boundaries.

---

## NFR-008 — Observability

Every Resolve-X service shall itself expose:

* Logs
* Metrics
* Traces
* Health information

Resolve-X must be observable using the same principles it provides to customers.

---

## NFR-009 — Fault Isolation

Failure in AI analysis must not prevent:

* Telemetry ingestion
* Incident creation
* Existing incident visibility

AI should be an enhancement to the incident pipeline, not a single point of failure.

---

## NFR-010 — Explainability

AI-generated conclusions should contain supporting evidence.

---

# 3. Actors

| Actor                  | Description                                                                 | Primary Interactions                                           |
| ---------------------- | --------------------------------------------------------------------------- | -------------------------------------------------------------- |
| Organization Admin     | Manages a tenant's users, projects, environments and API credentials.       | FR-001, FR-002, FR-003, NFR-006                                |
| Software Engineer      | Owns one or more services and investigates failures in them.                | FR-005, FR-013, FR-016, FR-018                                 |
| SRE / Platform Engineer| Operates the platform, onboards workloads and responds to incidents.        | FR-004, FR-010, FR-011, FR-012, FR-019                         |
| Engineering Lead       | Reviews incident history and recurring failure patterns.                    | FR-013, FR-016                                                 |
| Incident Responder     | Acknowledges, investigates, mitigates and resolves incidents.               | FR-012, FR-013, FR-014, FR-018, FR-019                         |
| Resolve-X Collector    | System actor that forwards telemetry and Kubernetes metadata.               | FR-004, FR-006, FR-007, FR-008, FR-009                         |
| External Alerting      | Alertmanager, PagerDuty or similar systems that raise alerts.               | FR-012                                                         |
| CI/CD System           | Pipelines that report deployment and configuration change events.           | FR-014, FR-015                                                 |
| LLM Provider           | External or self-hosted model used by the Analysis Service.                 | FR-016, FR-017                                                 |

---

# 4. MVP Priority

Requirements are prioritised for the first end-to-end milestone described in the [roadmap](roadmap.md).

| Priority   | Requirements                                                                                   |
| ---------- | ---------------------------------------------------------------------------------------------- |
| Must       | FR-001, FR-002, FR-003, FR-004, FR-005, FR-006, FR-007, FR-008, FR-009, FR-010, FR-011, FR-012, FR-013 |
| Should     | FR-014, FR-015, FR-016, FR-017, FR-018, FR-019                                                 |
| Later      | FR-020                                                                                         |

All non-functional requirements apply from the first release, although targets such as NFR-001 and NFR-005 will be validated progressively.

---

# 5. Related Documents

* [Use cases](use-cases.md)
* [Roadmap](roadmap.md)
* [System architecture](../architecture/system-architecture.md)
* [Architecture decision records](../adr/README.md)
