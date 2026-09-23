# ADR-004: Use Redis for Transient State

## Status

Accepted

## Context

Some Resolve-X operations require extremely frequent reads and short-lived state.

Examples include:

* Incident deduplication
* Rate limiting
* Correlation state
* Hot caches
* Distributed locks

## Decision

Redis will be used for transient and performance-sensitive state.

## Important Constraint

Redis will not be treated as the authoritative system of record for incidents or other critical domain data.

## Consequences

Redis failure should not result in permanent loss of core business data.
