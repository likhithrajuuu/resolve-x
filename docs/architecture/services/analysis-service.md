# Analysis Service

## Purpose

Produce an explainable, evidence-backed probable root cause and recommended actions for an incident.

It reasons over the evidence set built by the Correlation Service. It does not explore raw telemetry (ADR-008).

---

## Responsibilities

* Build the analysis context from incident, evidence and dependencies.
* Call the configured LLM provider.
* Enforce structured output.
* Validate that conclusions reference real evidence.
* Persist the analysis.
* Produce recommendations for human review (ADR-009).

---

## Does Not Own

* Incident state.
* Evidence selection.
* Execution of remediation.

---

## Processing Pipeline

```text
analysis.requested
      │
      ▼
Load incident + evidence set
      │
      ▼
Build context (bounded size, single tenant)
      │
      ▼
Redact configured sensitive fields
      │
      ▼
Call LLM provider (timeout, retry, rate limit)
      │
      ▼
Parse structured output
      │
      ▼
Validate
  ├── schema valid?
  ├── every cited evidence ID exists?
  └── confidence within range?
      │
      ▼
Persist analysis → analysis.completed
```

If validation fails, the service retries once with the validation errors, then marks the analysis `FAILED`. Incidents are unaffected.

---

## Output Contract

```json
{
  "summary": "Payment failures began after payment-service v2.8 was deployed and the database connection pool became exhausted.",
  "observedEvidence": ["ev_1", "ev_2", "ev_3"],
  "probableCauses": [
    {
      "cause": "Database connection pool exhaustion in payment-service",
      "confidence": 0.78,
      "supportingEvidence": ["ev_2", "ev_3"],
      "reasoning": "Pool exhaustion errors coincide with the DB latency increase and precede the 5xx increase."
    },
    {
      "cause": "Regression introduced in payment-service v2.8",
      "confidence": 0.64,
      "supportingEvidence": ["ev_1"],
      "reasoning": "The deployment occurred 6 seconds before the first symptom."
    }
  ],
  "uncertainty": "No configuration diff for v2.8 was available, so the pool-size change is not confirmed.",
  "recommendations": [
    {
      "action": "Compare connection pool configuration between v2.7 and v2.8.",
      "type": "investigate",
      "risk": "none"
    },
    {
      "action": "Roll back payment-service to v2.7 if the configuration change is confirmed.",
      "type": "remediate",
      "risk": "medium",
      "requiresApproval": true
    }
  ]
}
```

This expands the example result in [system-architecture.md](../system-architecture.md) to satisfy FR-016: observed evidence, inferred relationship, probable cause and uncertainty are separate fields.

---

## LLM Provider Abstraction

```text
AnalysisProvider
  ├── analyze(context) → structured result
  ├── name / model
  └── limits (tokens, rate)
```

The provider, model and whether a tenant permits third-party providers are configuration. The detailed abstraction is an open decision in the [ADR index](../../adr/README.md#open-decisions).

---

## Events

| Direction | Topic                 |
| --------- | --------------------- |
| Consumes  | `analysis.requested`  |
| Produces  | `analysis.completed`  |

---

## Data

```text
analyses
--------
id
tenant_id
incident_id
evidence_set_id
status            (PENDING | COMPLETED | FAILED)
provider
model
prompt_version
result (jsonb)
input_tokens
output_tokens
latency_ms
created_at
feedback          (helpful | incorrect | null)
```

Storing `prompt_version`, `provider` and `model` makes analyses auditable and comparable over time.

---

## Evaluation

A suite of known failure scenarios (roadmap M6) is run against every prompt or model change:

```text
scenario → evidence set → analysis → expected cause identified? → evidence cited correctly?
```

User feedback (`helpful` / `incorrect`) adds real cases to the suite.

---

## Failure Behaviour

| Failure                   | Behaviour                                                |
| ------------------------- | -------------------------------------------------------- |
| LLM provider down or slow | Retry with backoff; then `FAILED`, retry available later |
| Invalid output            | One corrective retry; then `FAILED`                      |
| Service down              | `analysis.requested` waits in Kafka; incidents unaffected |

---

## Observability

* Analyses by status.
* LLM latency, token usage and cost per tenant.
* Validation failure rate.
* User feedback rate.
