# Resolve-X — Use Cases

This document describes how actors interact with Resolve-X.

Actors are defined in [requirements.md](requirements.md#3-actors).

Each use case references the functional requirements it exercises.

---

## UC-001 — Onboard an Organization

**Actor:** Organization Admin

**Requirements:** FR-001, FR-002, FR-003, NFR-006

### Preconditions

* The admin has access to Resolve-X.

### Main Flow

```text
1. Admin creates an organization (tenant).
2. Admin creates a project, for example "Payments".
3. Admin creates environments: development, staging, production.
4. Admin generates an ingestion API key scoped to a project and environment.
5. Resolve-X stores the key securely and shows it once.
```

### Postconditions

* The tenant, project and environments exist.
* An ingestion credential is available for collector configuration.

### Alternative Flows

* **Key compromised:** the admin revokes the key and generates a new one. Telemetry sent with the revoked key is rejected.

---

## UC-002 — Connect an Application

**Actor:** SRE / Platform Engineer

**Requirements:** FR-004, FR-006, NFR-003

### Preconditions

* UC-001 is complete.
* The application runs on Kubernetes.

### Main Flow

```text
1. Engineer installs the Resolve-X Collector into the cluster.
2. Engineer configures the collector with the ingestion API key.
3. Engineer enables OpenTelemetry auto-instrumentation for the workload.
4. The application emits logs, metrics and traces over OTLP.
5. The collector enriches telemetry with Kubernetes metadata.
6. The collector forwards telemetry to the Ingestion Service.
7. The Ingestion Service authenticates, validates and accepts the telemetry.
```

### Postconditions

* Telemetry for the application is flowing into Resolve-X.

### Alternative Flows

* **Resolve-X unreachable:** the collector buffers telemetry locally and retries. The application is not affected.
* **Invalid credential:** the Ingestion Service rejects the request and the collector reports the error through its own logs and metrics.

---

## UC-003 — Discover Services Automatically

**Actor:** Resolve-X (system)

**Requirements:** FR-004, FR-005

### Main Flow

```text
1. Telemetry arrives with resource attributes such as
   service.name, service.version and deployment.environment.
2. The discovery engine resolves a canonical service identity.
3. If the service is unknown, it is registered.
4. If the service is known, its metadata and last-seen time are updated.
5. A service.discovered or service.updated event is published.
```

### Postconditions

* The service appears in the Service Registry without manual registration.

### Alternative Flows

* **Missing service.name:** the service is identified from Kubernetes workload metadata where possible, otherwise marked as `unknown_service` for review.
* **New version observed:** the version is recorded and a change event is added to the service history.

---

## UC-004 — View the Service Dependency Graph

**Actor:** Software Engineer, SRE / Platform Engineer

**Requirements:** FR-010, FR-011

### Main Flow

```text
1. Engineer opens a project and environment in the dashboard.
2. Resolve-X shows services and their observed dependencies.
3. Engineer selects a dependency edge.
4. Resolve-X shows request count, error count, latency
   and first/last observed time for that edge.
```

### Postconditions

* The engineer understands which components a service depends on and how those dependencies are behaving.

---

## UC-005 — Automatically Create an Incident

**Actor:** Resolve-X (system), External Alerting

**Requirements:** FR-012, FR-013, NFR-004, NFR-005

### Main Flow

```text
1. An alert, threshold violation or anomaly is received.
2. The Incident Service checks for an existing open incident
   with the same deduplication key.
3. No matching incident exists, so a new incident is created
   with severity, affected service and start time.
4. The incident timeline records the triggering signal.
5. incident.created is published.
6. Correlation is requested for the investigation window.
```

### Postconditions

* An incident exists and is visible to responders within the latency target.

### Alternative Flows

* **Duplicate alert:** the signal is attached to the existing incident timeline instead of creating a new incident.
* **Correlation or analysis unavailable:** the incident is still created and remains visible (NFR-009).

---

## UC-006 — Investigate an Incident

**Actor:** Incident Responder

**Requirements:** FR-013, FR-014, FR-015, FR-016, FR-017, NFR-010

### Main Flow

```text
1. Responder opens the incident.
2. Resolve-X shows:
   - affected services and impact
   - the incident timeline
   - correlated logs, metrics and traces
   - recent deployments and configuration changes
   - relevant dependencies
3. Resolve-X shows the AI analysis:
   - observed evidence
   - inferred relationships
   - probable causes with confidence
   - stated uncertainty
4. Responder follows evidence links to the underlying telemetry.
5. Responder moves the incident to IDENTIFIED.
```

### Postconditions

* The responder has a probable cause supported by evidence.

### Alternative Flows

* **AI analysis pending:** the incident shows correlated evidence and marks analysis as `PENDING`.
* **Analysis disputed:** the responder marks the analysis as incorrect. The feedback is stored for evaluation.

---

## UC-007 — Review and Approve a Remediation

**Actor:** Incident Responder

**Requirements:** FR-018, FR-019

### Main Flow

```text
1. Resolve-X presents recommended actions.
2. Responder reviews a recommendation.
3. Responder approves, rejects or modifies it.
4. The decision is recorded on the incident timeline with the user identity.
5. If the action is executed, the result is recorded.
```

### Postconditions

* Every executed action has a recorded approver and outcome.

### Business Rule

* Destructive actions are never executed automatically unless automated remediation has been explicitly configured and authorized (ADR-009).

---

## UC-008 — Resolve and Close an Incident

**Actor:** Incident Responder

**Requirements:** FR-013

### Main Flow

```text
1. Responder confirms mitigation.
2. Incident moves to RESOLVED.
3. Resolve-X continues to observe the affected signals.
4. After confirmation, the incident moves to CLOSED.
```

### Alternative Flows

* **Recurrence:** if the triggering signal returns within the observation window, the incident is reopened rather than duplicated.

---

## UC-009 — Correlate a Deployment With an Incident

**Actor:** CI/CD System, Resolve-X (system)

**Requirements:** FR-014, FR-015

### Main Flow

```text
1. The CI/CD system or Kubernetes reports a deployment
   for payment-service v2.8.
2. Resolve-X records the deployment against the service.
3. An incident later occurs on payment-service.
4. The correlation engine finds the deployment inside
   the investigation window.
5. The deployment is added to the incident evidence
   and timeline.
```

### Postconditions

* Responders can see what changed before the incident.

---

## UC-010 — Review Incident History

**Actor:** Engineering Lead

**Requirements:** FR-013, FR-016

### Main Flow

```text
1. Lead filters closed incidents by project, service and time range.
2. Resolve-X lists incidents with severity, duration and probable cause.
3. Lead identifies services or causes that recur.
```

### Postconditions

* Recurring failure patterns are visible.

---

## UC-011 — Monitor Resolve-X Itself

**Actor:** SRE / Platform Engineer (operating Resolve-X)

**Requirements:** NFR-008, ADR-013

### Main Flow

```text
1. Each Resolve-X service emits logs, metrics and traces.
2. Resolve-X telemetry is ingested into an internal tenant.
3. Resolve-X services appear in its own Service Registry.
4. Failures in Resolve-X produce incidents like any other workload.
```

### Postconditions

* Resolve-X is observable using its own pipeline.
