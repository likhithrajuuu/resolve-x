# API Gateway

## Purpose

The single external entry point for the dashboard, users and integrations.

---

## Responsibilities

* Authenticate users and API clients.
* Authorize requests against tenant, project and role.
* Route requests to internal services.
* Rate limit per tenant and per credential.
* Validate requests at the edge.
* Propagate tenant context and trace context to downstream services.

Initially also hosts tenant, user, project, environment and API key management (see [container diagram](../container-diagram.md#2-containers)).

---

## Does Not Own

* Telemetry ingestion. OTLP traffic goes directly to the Ingestion Service, which has very different load characteristics.
* Business logic of downstream services.

---

## API (initial design)

### Tenancy and access

```text
POST   /api/v1/projects
GET    /api/v1/projects
POST   /api/v1/projects/{projectId}/environments
GET    /api/v1/projects/{projectId}/environments
POST   /api/v1/projects/{projectId}/api-keys
DELETE /api/v1/projects/{projectId}/api-keys/{keyId}
```

### Routed

```text
/api/v1/services/**        → Service Registry
/api/v1/dependencies/**    → Service Registry
/api/v1/incidents/**       → Incident Service
/api/v1/analyses/**        → Analysis Service
/api/v1/webhooks/alerts    → Incident Service
/api/v1/webhooks/deployments → Service Registry
```

---

## Data

```text
tenants
users
memberships (user, tenant, role)
projects
environments
api_keys (hashed, with scope and status)
audit_log
```

API keys are stored hashed and shown only once at creation.

---

## Security

* All external traffic over TLS.
* Tenant ID is derived from the authenticated identity, never from a request parameter the caller controls.
* Administrative actions are written to the audit log (NFR-006).

---

## Failure Behaviour

* If a downstream service is unavailable, the gateway returns an error for that route only; other routes keep working.
* Timeouts and circuit breakers are applied per downstream service.

---

## Observability

* Request rate, error rate and latency per route and tenant.
* Authentication failures and rate-limit rejections.
