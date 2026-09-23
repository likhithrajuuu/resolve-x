# ADR-006: Microservice Boundaries

## Status

Accepted

## Context

Resolve-X itself is a distributed system and should demonstrate appropriate service boundaries.

However, excessive microservices would increase development and operational complexity.

## Decision

The initial logical boundaries are:

```text
API Gateway
Ingestion Service
Service Registry
Incident Service
Correlation Service
Analysis Service
```

These boundaries are based on responsibilities and scaling characteristics rather than simply creating one service per entity.

## Principle

A service should exist when there is a meaningful reason involving:

* Independent scaling
* Independent failure isolation
* Clear ownership
* Different processing characteristics
* Separate lifecycle

## Consequence

The architecture remains capable of evolving without starting with an unnecessarily large number of deployables.
