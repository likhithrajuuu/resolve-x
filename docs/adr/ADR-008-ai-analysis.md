# ADR-008: Evidence-First AI Analysis

## Status

Accepted

## Context

An LLM should not be given unrestricted responsibility for discovering the cause of an incident.

Raw telemetry can be enormous, noisy and difficult to interpret.

## Decision

Resolve-X will use an evidence-first architecture.

```text
Raw Telemetry
      ↓
Normalization
      ↓
Correlation
      ↓
Evidence Selection
      ↓
Incident Context
      ↓
AI Analysis
```

The AI service receives structured evidence.

## Example

```text
Incident:
Payment failures

Evidence:

Metric:
5xx rate increased from 1% → 18%

Trace:
95% of latency occurs in PostgreSQL

Logs:
Connection pool exhausted

Deployment:
payment-service v2.8 deployed 8 minutes before incident
```

The AI then reasons over this evidence.

## Consequences

### Positive

* More explainable analysis
* Smaller prompts
* Lower AI cost
* Better grounding
* Easier testing
* Easier auditing

### Negative

* Correlation engine becomes important
* Evidence-selection logic must be maintained
* AI may still produce incorrect conclusions
