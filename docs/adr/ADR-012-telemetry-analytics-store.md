# ADR-012: PostgreSQL Is Not the Primary Telemetry Analytics Store

## Status

Accepted

## Context

Core transactional data and high-volume telemetry have different workload characteristics.

## Decision

PostgreSQL will store transactional domain data.

A specialized analytical system such as ClickHouse may store large historical telemetry datasets when required.

```text
PostgreSQL
    ↓
Transactions

ClickHouse
    ↓
Large-scale analytical telemetry
```

This separation prevents telemetry workloads from overwhelming transactional workloads.
