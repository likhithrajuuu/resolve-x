# Architecture Decision Records

This directory contains the Architecture Decision Records (ADRs) for Resolve-X.

An ADR captures a single significant architectural decision, the context in which it was made, the alternatives considered and the consequences of the decision.

Technology choices in Resolve-X must be supported by a documented decision.

---

## Index

| ADR                                                    | Title                                                 | Status   |
| ------------------------------------------------------ | ----------------------------------------------------- | -------- |
| [ADR-001](ADR-001-opentelemetry.md)                    | Use OpenTelemetry for Telemetry                       | Accepted |
| [ADR-002](ADR-002-kafka.md)                            | Use Kafka for Asynchronous Event Processing           | Accepted |
| [ADR-003](ADR-003-postgresql.md)                       | Use PostgreSQL for Transactional Data                 | Accepted |
| [ADR-004](ADR-004-redis.md)                            | Use Redis for Transient State                         | Accepted |
| [ADR-005](ADR-005-service-discovery.md)                | Service Discovery Strategy                            | Accepted |
| [ADR-006](ADR-006-service-boundaries.md)               | Microservice Boundaries                               | Accepted |
| [ADR-007](ADR-007-rest-vs-rpc.md)                      | REST vs gRPC vs Kafka                                 | Accepted |
| [ADR-008](ADR-008-ai-analysis.md)                      | Evidence-First AI Analysis                            | Accepted |
| [ADR-009](ADR-009-human-approval-remediation.md)       | Human Approval for Remediation                        | Accepted |
| [ADR-010](ADR-010-multi-tenancy.md)                    | Multi-Tenant Architecture                             | Accepted |
| [ADR-011](ADR-011-collector-integration.md)            | Agent/Collector-Based Customer Integration            | Accepted |
| [ADR-012](ADR-012-telemetry-analytics-store.md)        | PostgreSQL Is Not the Primary Telemetry Analytics Store | Accepted |
| [ADR-013](ADR-013-self-observability.md)               | Resolve-X Must Be Observable                          | Accepted |

---

## Open Decisions

The following decisions are known to be required but have not yet been made.

| Topic                           | Notes                                                                 |
| ------------------------------- | --------------------------------------------------------------------- |
| Frontend technology             | Listed as "to be determined" in the README.                           |
| Event serialization format      | Protobuf, Avro or JSON for Kafka payloads; schema registry usage.     |
| Authentication provider         | Self-managed identity vs external OIDC provider.                      |
| LLM provider abstraction        | Provider interface, fallback strategy and data-handling policy.       |
| Anomaly detection approach      | Static thresholds, statistical baselines or learned models.           |
| Telemetry storage backends      | When to introduce Loki, Tempo, Prometheus and ClickHouse in the MVP.  |
| Deployment event source         | Kubernetes rollout events, CI/CD webhooks or both.                    |

---

## Process

1. Copy [`template.md`](template.md) to `ADR-NNN-short-title.md` using the next available number.
2. Set the status to `Proposed`.
3. Describe the context, decision, alternatives and consequences.
4. Discuss and review the ADR.
5. Change the status to `Accepted` or `Rejected`.

ADRs are not edited to reflect later changes of direction.

When a decision changes, a new ADR is written and the original is marked:

```text
Superseded by ADR-NNN
```

---

## Statuses

```text
Proposed
Accepted
Rejected
Deprecated
Superseded by ADR-NNN
```
