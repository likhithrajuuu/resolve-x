# ADR-015: Derived Service Registry and Merged Correlation/Analysis

## Status

Accepted (supersedes the initial split in ADR-006 for the first release)

## Context

ADR-006 separates Service Registry, Correlation Service and Analysis Service. Building all three as independent deployables before there is a user for them adds operational cost without changing behaviour.

## Decision

* **Service catalog and dependency graph are derived from spans at query time** by the Query service (ClickHouse), not stored in a registry with its own tables. Services appear without registration, which is the product promise.
* **Correlation and analysis run in one `analysis` service.** Correlation is a deterministic gather step; analysis ranks causes and optionally asks an LLM. They share one process and one Kafka consumer (`incident.created`, `analysis.requested` → `analysis.completed`).
* **The anomaly detector lives in the Incident service**, because it creates incidents and already needs the dedup index.

## Consequences

* No `service.discovered` events and no registry-owned deployment history yet; deployments are stored by the Incident service.
* Dependency edges are recomputed on every request. Materialising them is the first thing to do if that becomes slow.
* The three concerns can still be split later; their boundaries are package boundaries (`internal/query`, `internal/analysis`, `internal/incident`) and Kafka topics.
