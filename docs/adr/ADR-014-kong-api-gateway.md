# ADR-014: Use Kong Gateway (OSS) as the API Gateway

## Status

Accepted

## Context

The first gateway was a hand-written Go reverse proxy. Routing, rate limiting, TLS, CORS and request correlation are commodity concerns and are better handled by a maintained gateway.

## Decision

Kong Gateway open source, DB-less, configured declaratively in `backend/deploy/kong/kong.yml`.

* Kong owns routing, rate limiting, CORS, request IDs, request size limits and metrics.
* The Go gateway became the **tenancy service** (projects, environments, API keys, audit log). It sits behind Kong and still authenticates the caller and derives the tenant itself.
* OTLP traffic is **not** routed through Kong; ingestion is exposed directly.

## Alternatives

* kgateway (Envoy, Kubernetes Gateway API): needs a cluster even for local development.
* Keep the custom Go proxy: more code to own for no differentiation.

## Consequences

* Kong OSS has no OIDC plugin; user authentication will use the `jwt` plugin or stay in the tenancy service until an identity provider is chosen.
* Per-tenant rate limits need consumer-based auth in Kong; today limits are per client IP.
* With more than one Kong replica, rate limiting must use the Redis policy.
