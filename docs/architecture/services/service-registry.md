# Service Registry

## Purpose

Maintain Resolve-X's understanding of which services exist, how they are deployed and what they depend on.

It is an observability catalog, not a replacement for Kubernetes service discovery (ADR-005).

---

## Responsibilities

* Resolve canonical service identity from telemetry resource attributes.
* Register new services and update known ones.
* Track versions, environments and first/last seen times.
* Track service status.
* Maintain the dependency graph from dependency observations.
* Record deployments and configuration changes.

---

## Does Not Own

* Raw telemetry.
* Incidents.

---

## Service Identity

A service is identified by:

```text
tenant_id + project_id + environment_id + service.namespace + service.name
```

Resolution order for the name:

```text
1. service.name resource attribute (if not the SDK default)
2. k8s.deployment.name / k8s.statefulset.name
3. unknown_service (flagged for review)
```

Language and framework come from `telemetry.sdk.language` and instrumentation scope metadata.

---

## Service Status (initial design)

```text
ACTIVE    telemetry seen within the activity window
INACTIVE  no telemetry for longer than the activity window
RETIRED   explicitly marked by a user
```

Health (healthy / degraded / failing) is derived from metrics and incidents and shown alongside status.

---

## API (initial design)

```text
GET /api/v1/services?projectId=&environmentId=&status=
GET /api/v1/services/{serviceId}
GET /api/v1/services/{serviceId}/versions
GET /api/v1/services/{serviceId}/deployments
GET /api/v1/dependencies/graph?projectId=&environmentId=
GET /api/v1/services/{serviceId}/dependencies?direction=upstream|downstream
POST /api/v1/webhooks/deployments
```

---

## Events

| Direction | Topic                  | Notes                                     |
| --------- | ---------------------- | ----------------------------------------- |
| Consumes  | `telemetry.raw`        | Resource attributes for discovery         |
| Consumes  | `dependency.observed`  | Aggregated edges from trace processing    |
| Consumes  | `deployment.recorded`  | Deployments from CI/CD or Kubernetes      |
| Produces  | `service.discovered`   | First time a service is seen              |
| Produces  | `service.updated`      | Version, metadata or status change        |

Discovery consumes `telemetry.raw` but only reads resource attributes. Updates are throttled so that the same service is not rewritten on every batch.

---

## Data

```text
services
---------
id
tenant_id
project_id
environment_id
namespace
name
language
framework
current_version
status
k8s_metadata (jsonb)
first_seen_at
last_seen_at

service_versions
----------------
service_id
version
first_seen_at
last_seen_at

dependencies
------------
id
tenant_id
source_service_id
target_service_id   (nullable for non-service targets)
target_name         (for databases, caches, external APIs)
type                (service | database | cache | message_broker | external_api)
first_seen_at
last_seen_at
request_count
error_count
latency_p50_ms
latency_p95_ms

deployments
-----------
id
tenant_id
service_id
version
source              (kubernetes | ci | manual)
deployed_at
metadata (jsonb)
```

---

## Failure Behaviour

* If the registry is down, telemetry ingestion continues. Discovery catches up from Kafka when it recovers.
* Service lookups used by other services are cached in Redis.

---

## Observability

* Services discovered and updated per tenant.
* Consumer lag on discovery topics.
* Count of `unknown_service` identities.
