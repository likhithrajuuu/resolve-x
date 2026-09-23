# Correlation Service

## Purpose

Turn an incident into an evidence set: the telemetry, changes and dependencies most likely to explain it.

This is the deterministic core that AI analysis depends on (ADR-008).

---

## Responsibilities

* Define the investigation window.
* Identify affected services and relevant dependencies.
* Gather related metrics, logs, traces and deployments.
* Detect temporal relationships between signals.
* Build the incident timeline entries.
* Rank and select evidence.
* Persist the evidence set.

---

## Does Not Own

* Incident state.
* Natural-language conclusions. It reports facts and scored relationships, not diagnoses.

---

## Correlation Process

```text
correlation.requested
        │
        ▼
1. Investigation window
   [incident start − lookback, now]      lookback initially 30 min
        │
        ▼
2. Scope
   primary service
   + upstream callers
   + downstream dependencies (from Service Registry)
        │
        ▼
3. Gather candidate signals
   ├── metric changes vs. baseline
   ├── error and warning logs
   ├── failed or slow traces
   ├── deployments and config changes
   └── infrastructure events (pod restarts, OOM kills)
        │
        ▼
4. Score each signal
        │
        ▼
5. Select top evidence within size limits
        │
        ▼
6. Persist evidence set → correlation.completed
```

---

## Correlation Dimensions

| Dimension   | Question                                                       |
| ----------- | -------------------------------------------------------------- |
| Time        | Did it happen shortly before or during the incident?           |
| Service     | Is it on the affected service or a direct dependency?          |
| Trace       | Does it appear in the same traces as the failing requests?     |
| Dependency  | Is it on the path between the symptom and a dependency?        |
| Deployment  | Did a change precede it?                                       |
| Error type  | Does it share an exception type or message pattern?            |
| Metric      | Did the metric deviate from its baseline?                      |

These map directly to FR-015.

---

## Scoring (initial approach)

Scoring starts rule-based and transparent:

```text
score = time proximity
      + topological proximity
      + deviation magnitude
      + trace co-occurrence
      + change proximity
```

Weights are configuration, not code, so they can be tuned against the evaluation scenarios in roadmap M6.

Learned models may be considered later, but only if they keep evidence explainable.

---

## Evidence Set

```json
{
  "evidenceSetId": "evs_789",
  "incidentId": "inc_123",
  "window": { "from": "2026-09-23T10:02:15Z", "to": "2026-09-23T10:40:00Z" },
  "scope": {
    "primaryService": "payment-service",
    "dependencies": ["postgresql:payments-db"],
    "callers": ["order-service"]
  },
  "items": [
    {
      "id": "ev_1",
      "type": "deployment",
      "service": "payment-service",
      "at": "2026-09-23T10:31:58Z",
      "summary": "payment-service v2.8 deployed (previous v2.7)",
      "score": 0.91,
      "source": { "kind": "deployment", "ref": "dep_555" }
    },
    {
      "id": "ev_2",
      "type": "metric_change",
      "service": "payments-db",
      "at": "2026-09-23T10:32:08Z",
      "summary": "DB latency p95 120 ms → 1.8 s",
      "score": 0.88,
      "source": { "kind": "metric", "ref": "db.client.operation.duration" }
    },
    {
      "id": "ev_3",
      "type": "log_pattern",
      "service": "payment-service",
      "at": "2026-09-23T10:32:10Z",
      "summary": "\"Connection pool exhausted\" × 412",
      "score": 0.86,
      "source": { "kind": "log", "ref": "sample log ids" }
    }
  ]
}
```

Every item has a `source` so the dashboard and the Analysis Service can link back to the underlying telemetry.

Items store a short excerpt, so incidents stay explainable after raw telemetry expires.

---

## API (initial design)

```text
GET  /api/v1/incidents/{incidentId}/evidence       (served via Incident Service)
POST /internal/v1/evidence-sets/{id}/recompute     re-run with a new window
```

---

## Events

| Direction | Topic                   |
| --------- | ----------------------- |
| Consumes  | `correlation.requested` |
| Consumes  | `deployment.recorded`   |
| Produces  | `correlation.completed` |

---

## Data

```text
evidence_sets (id, tenant_id, incident_id, window_from, window_to, scope, created_at)
evidence_items (id, evidence_set_id, type, service, occurred_at, summary, score, source)
```

Short-lived correlation state (for example baselines being computed) lives in Redis.

---

## Failure Behaviour

* If a telemetry store is unavailable, the evidence set is produced from the remaining sources and marked partial.
* Correlation can be re-run; results are keyed by incident and window, so reprocessing is idempotent.

---

## Observability

* Time to produce an evidence set.
* Evidence items per incident and per type.
* Partial evidence sets and their missing sources.
