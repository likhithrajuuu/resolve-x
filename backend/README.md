# Resolve-X backend

Go services, one binary per service under `cmd/`.

| Service | Port | Role |
| --- | --- | --- |
| `ingestion` | 4317 gRPC, 4318 HTTP | OTLP in, API-key auth, per-tenant rate limit, publish to `telemetry.raw` |
| `processor` | – | `telemetry.raw` → split topics + ClickHouse |
| `tenancy` | 8080 (internal) | Projects, environments, API keys, audit; behind Kong |
| Kong (OSS) | 8000 | Public API: routing, rate limiting, CORS, request IDs. Config: `deploy/kong/kong.yml` |

Every service exposes `/healthz`, `/readyz` and Prometheus `/metrics` on `:9100`.

```bash
make up      # Kafka, Postgres, Redis, ClickHouse + the three services
make load    # synthetic OTLP load (dev key: rx_dev_local_key)
make down
```

Tenancy dev auth (via Kong on :8000): `Authorization: Bearer dev-token` (never use outside local dev).

## Scaling notes

- Ingestion is stateless; scale replicas on CPU (`deploy/k8s/ingestion.yaml`).
- `telemetry.raw` records are unkeyed to spread load evenly; the processor re-keys by `tenant:service`.
- Kafka retains data so consumers can lag during spikes without back-pressuring ingestion.
- Processing is at-least-once; ClickHouse rows can duplicate after a crash (dedup is a later step).
- Rate limits are per instance; a global Redis limiter is a later step.

Not built yet: service-registry, incident, correlation and analysis services, OIDC auth, outbox relay, schema registry.
