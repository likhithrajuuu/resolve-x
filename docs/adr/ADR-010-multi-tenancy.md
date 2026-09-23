# ADR-010: Multi-Tenant Architecture

## Status

Accepted

## Context

Resolve-X is intended to be a platform rather than a single-organization monitoring application.

Multiple organizations must coexist while maintaining strict data isolation.

## Decision

Tenant context will be part of the core data model.

```text
Tenant
  │
  ├── Projects
  │      │
  │      ├── Environments
  │      │      │
  │      │      ├── Services
  │      │      └── Incidents
  │      │
  │      └── API Credentials
  │
  └── Users
```

Every telemetry ingestion request must be associated with an authenticated tenant/project context.

## Consequences

Multi-tenancy must be considered in:

* Database schema
* APIs
* Kafka events
* Cache keys
* Authorization
* Telemetry storage
* AI analysis
* Audit logs
