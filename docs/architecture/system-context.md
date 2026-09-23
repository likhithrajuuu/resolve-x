# Resolve-X — System Context

This document describes Resolve-X as a single system and the people and external systems it interacts with.

It corresponds to level 1 of the C4 model.

The internal structure is described in the [container diagram](container-diagram.md).

---

## 1. Context Diagram

```text
      ┌───────────────────┐   ┌───────────────────┐   ┌───────────────────┐
      │ Software Engineer │   │ SRE / Platform    │   │ Engineering Lead  │
      │                   │   │ Engineer          │   │                   │
      └─────────┬─────────┘   └─────────┬─────────┘   └─────────┬─────────┘
                │   Investigates        │   Operates,           │   Reviews
                │   incidents           │   onboards            │   history
                └───────────────────────┼───────────────────────┘
                                        │ HTTPS (Dashboard / REST)
                                        ▼
┌──────────────────┐  OTLP     ┌─────────────────────────┐   HTTPS    ┌──────────────────┐
│ Customer         │─────────▶ │                         │ ─────────▶ │ LLM Provider     │
│ Applications     │ (via      │                         │            │                  │
│ (Java, Python,   │ collector)│        RESOLVE-X        │            └──────────────────┘
│  Go, Node.js)    │           │                         │
└──────────────────┘           │  Discovery              │   Webhook  ┌──────────────────┐
                               │  Correlation            │ ─────────▶ │ Notification     │
┌──────────────────┐ Metadata  │  Incident management    │            │ Channels         │
│ Kubernetes       │─────────▶ │  AI-assisted analysis   │            │ (Slack, Webhooks)│
│ Clusters         │ (via      │                         │            └──────────────────┘
└──────────────────┘ collector)│                         │
                               │                         │
┌──────────────────┐ Alerts    │                         │
│ External         │─────────▶ │                         │
│ Alerting         │ (webhook) │                         │
└──────────────────┘           │                         │
                               │                         │
┌──────────────────┐ Deploy    │                         │
│ CI/CD Systems    │─────────▶ │                         │
│                  │ events    │                         │
└──────────────────┘           └─────────────────────────┘
```

---

## 2. People

| Actor                   | Relationship to Resolve-X                                                   |
| ----------------------- | --------------------------------------------------------------------------- |
| Software Engineer       | Investigates incidents affecting the services they own.                     |
| SRE / Platform Engineer | Installs collectors, onboards workloads and responds to incidents.          |
| Engineering Lead        | Reviews incident history and recurring failure patterns.                    |
| Organization Admin      | Manages users, projects, environments and credentials.                      |

---

## 3. External Systems

### Customer Applications

The systems being observed.

They emit logs, metrics and traces using OpenTelemetry SDKs or auto-instrumentation (ADR-001).

Applications never call Resolve-X directly. Telemetry goes through a collector (ADR-011).

### Kubernetes Clusters

Provide workload metadata such as namespace, deployment, pod, node and labels.

This metadata is attached to telemetry by the collector and used for service discovery (ADR-005).

### External Alerting

Existing alerting systems such as Prometheus Alertmanager or PagerDuty.

Their alerts can create or update Resolve-X incidents (FR-012).

### CI/CD Systems

Report deployments and configuration changes.

These events are key evidence when correlating incidents (FR-014).

### LLM Provider

A pluggable external or self-hosted model used only by the Analysis Service.

It receives a curated incident context, never unrestricted telemetry (ADR-008).

### Notification Channels

Receive incident notifications.

Planned for after the MVP (FR-020).

---

## 4. Trust Boundaries

```text
┌──────────────── Customer network ────────────────┐
│  Applications ──▶ Resolve-X Collector            │
└──────────────────────────┬───────────────────────┘
                           │ TLS + API key
┌──────────────────────────▼─── Resolve-X ─────────┐
│  Ingestion ──▶ internal services ──▶ data stores │
└──────────────────────────┬───────────────────────┘
                           │ TLS, minimal context
┌──────────────────────────▼───────────────────────┐
│  LLM Provider (third party or self-hosted)       │
└──────────────────────────────────────────────────┘
```

Key considerations:

* All traffic crossing a boundary is encrypted (NFR-006).
* Every ingestion request is authenticated and bound to a tenant (ADR-010).
* Telemetry may contain sensitive data. Only the evidence required for analysis is sent to the LLM provider.
* Whether a tenant's data may be sent to a third-party LLM should be configurable per tenant.

---

## 5. Key Qualities at This Level

| Quality          | Expectation                                                                  |
| ---------------- | ---------------------------------------------------------------------------- |
| Non-intrusive    | Customer applications keep working if Resolve-X is unavailable.              |
| Multi-tenant     | Tenants are strictly isolated.                                               |
| Explainable      | Every conclusion links back to evidence.                                     |
| Human controlled | Resolve-X recommends; people approve.                                        |
