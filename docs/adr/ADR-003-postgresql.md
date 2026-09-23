# ADR-003: Use PostgreSQL for Transactional Data

## Status

Accepted

## Context

Resolve-X requires strongly structured transactional data such as tenants, projects, services, incidents and users.

## Decision

PostgreSQL will be the primary transactional database.

## Data

```text
tenants
users
projects
environments
services
dependencies
incidents
incident_timeline
analyses
api_keys
```

## Alternatives

* MongoDB
* MySQL
* PostgreSQL
* Distributed SQL database

## Rationale

Resolve-X has relational entities and relationships that benefit from transactional guarantees and SQL querying.

PostgreSQL also provides a mature ecosystem and supports future extensions where required.

## Consequences

PostgreSQL becomes the system of record for core transactional data.

High-volume analytical telemetry will not be forced into the transactional database.
