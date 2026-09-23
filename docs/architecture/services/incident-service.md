# Incident Service

## Purpose

Own the incident lifecycle: creation, deduplication, state, timeline and resolution.

This is the system of record for incidents. It must keep working when correlation or analysis is unavailable (NFR-009).

---

## Responsibilities

* Create incidents from alerts, thresholds, anomalies, integrations and users.
* Deduplicate triggers into existing incidents.
* Manage severity, status and assignment.
* Maintain the incident timeline.
* Request correlation and analysis.
* Attach evidence and analysis references.
* Record remediation decisions (ADR-009).

---

## Does Not Own

* Evidence gathering (Correlation Service).
* AI analysis (Analysis Service).
* Telemetry.

---

## Lifecycle

```text
OPEN
  ↓
INVESTIGATING
  ↓
IDENTIFIED
  ↓
MITIGATING
  ↓
RESOLVED
  ↓
CLOSED
```

Rules (initial design):

* Any active state may move directly to `RESOLVED`.
* `RESOLVED` may return to `INVESTIGATING` if the trigger recurs within the observation window.
* `CLOSED` is final; recurrence after closing creates a new incident linked to the old one.
* Every transition is recorded on the timeline with the actor.

---

## Severity

```text
SEV1  critical  — major user-facing outage
SEV2  high      — significant degradation
SEV3  medium    — limited impact
SEV4  low       — minor or internal
```

---

## Deduplication

```text
dedup key = tenant + environment + service + trigger type (+ alert fingerprint)
```

The key is held in Redis while the incident is active. PostgreSQL remains authoritative; if Redis is lost, the key is rebuilt from open incidents (ADR-004).

---

## API (initial design)

```text
POST  /api/v1/incidents                          manual creation
GET   /api/v1/incidents?status=&severity=&serviceId=&from=&to=
GET   /api/v1/incidents/{incidentId}
PATCH /api/v1/incidents/{incidentId}             status, severity, assignee
GET   /api/v1/incidents/{incidentId}/timeline
POST  /api/v1/incidents/{incidentId}/timeline    add a note
GET   /api/v1/incidents/{incidentId}/evidence
POST  /api/v1/incidents/{incidentId}/actions/{actionId}/approve
POST  /api/v1/incidents/{incidentId}/actions/{actionId}/reject
POST  /api/v1/webhooks/alerts                    external alerts
```

---

## Events

| Direction | Topic                   |
| --------- | ----------------------- |
| Produces  | `incident.created`      |
| Produces  | `incident.updated`      |
| Produces  | `correlation.requested` |
| Produces  | `analysis.requested`    |
| Consumes  | `correlation.completed` |
| Consumes  | `analysis.completed`    |
| Consumes  | `service.updated`       |

Events are published through a transactional outbox (see [data-flow.md](../data-flow.md#4-delivery-rules)).

---

## Data

```text
incidents
---------
id
tenant_id
project_id
environment_id
title
severity
status
trigger_type        (alert | threshold | anomaly | integration | manual)
dedup_key
primary_service_id
affected_service_ids
started_at
detected_at
resolved_at
closed_at
assignee_id
evidence_set_id
latest_analysis_id
analysis_status     (NOT_REQUESTED | PENDING | COMPLETED | FAILED)

incident_timeline
-----------------
id
incident_id
occurred_at
type                (signal | status_change | deployment | evidence | analysis | note | action)
actor               (system | user id)
summary
reference (jsonb)

remediation_actions
-------------------
id
incident_id
source              (analysis | user)
description
status              (RECOMMENDED | APPROVED | REJECTED | EXECUTED | FAILED)
decided_by
decided_at
result
```

---

## Failure Behaviour

| Dependency down      | Behaviour                                                      |
| -------------------- | -------------------------------------------------------------- |
| Correlation Service  | Incident created; evidence shown as pending.                   |
| Analysis Service     | Incident created; `analysis_status = PENDING`.                 |
| Redis                | Deduplication falls back to a PostgreSQL query.                |
| Kafka                | Outbox holds events until Kafka recovers.                      |

---

## Observability

* Incidents created per tenant and trigger type.
* Time from trigger to incident creation (target < 5 s, NFR-005).
* Deduplication hit rate.
* Outbox backlog.
