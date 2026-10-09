# Resolve-X backend

Go services, one binary per service under `cmd/`. Public traffic enters through Kong (`:8000`); OTLP goes straight to ingestion.

| Service | Port | Role |
| --- | --- | --- |
| Kong (OSS) | 8000 | Public API: routing, rate limiting, CORS, request IDs (`deploy/kong/kong.yml`) |
| `ingestion` | 4317 gRPC, 4318 HTTP | OTLP in, API-key auth, per-tenant rate limit, publish to `telemetry.raw` |
| `processor` | – | `telemetry.raw` → per-signal topics + ClickHouse |
| `tenancy` | 8080 (internal) | Signup/login (JWT), projects, environments, API keys, audit log |
| `query` | 8081 (internal) | Services, dependency graph, traces, logs, metrics (ClickHouse) |
| `incident` | 8082 (internal) | Incidents, timeline, alert/deployment webhooks, auto-detector, outbox relay |
| `analysis` | – | Correlates evidence and finds the probable root cause (heuristics, optional Claude) |

Every service exposes `/healthz`, `/readyz` and Prometheus `/metrics` on `:9100`.

## Run it

```bash
make up                # Kafka, Postgres, Redis, ClickHouse + all services
make seed              # healthy traffic (adds history so baselines exist)
make incident-demo     # payments fault + deployment -> incident -> analysis
make e2e               # automated check of the whole product
make down              # stop and delete data
```

Dev login: `dev@resolve-x.local` / `devpassword1` (seeded by `DEV_SEED_USER`; never set it in production). Dev ingestion key: `rx_dev_local_key`.

## How an incident happens

1. Services send OTLP to ingestion → Kafka `telemetry.raw` → processor → ClickHouse.
2. The detector (in `incident`) checks the last 5 minutes per service: error rate ≥ 10%, or p95 ≥ 3× the previous hour and ≥ 500 ms. If a service's dependencies are also failing, only the deepest failing service gets an incident.
3. The incident and an `incident.created` event commit in one transaction (outbox). The relay publishes it.
4. `analysis` gathers evidence (service stats vs baseline, degraded dependencies, error logs, failing spans, recent deployments), ranks causes, and publishes `analysis.completed`.
5. `incident` stores the analysis. A person approves or rejects the recommended action. **Resolve-X records the decision; it never changes your systems.**

Set `ANTHROPIC_API_KEY` (and optionally `ANTHROPIC_MODEL`) before `make up` to let Claude refine the heuristic result. **That sends correlated evidence, including log text, to the Anthropic API.** Unset, nothing leaves your environment.

## Webhooks (authenticated with a project API key)

```bash
curl -XPOST localhost:8000/api/v1/webhooks/deployments -H 'X-Resolvex-Key: <key>' \
  -H 'Content-Type: application/json' -d '{"service":"payments","version":"v2.14.0"}'
curl -XPOST localhost:8000/api/v1/webhooks/alerts -H 'X-Resolvex-Key: <key>' \
  -H 'Content-Type: application/json' -d '{"title":"p99 high","service":"payments","severity":"critical"}'
```
Prometheus Alertmanager payloads are accepted on the alerts webhook as well.

## Scaling notes

- Ingestion is stateless; scale replicas on CPU (`deploy/k8s/ingestion.yaml`).
- `telemetry.raw` records are unkeyed to spread load; the processor re-keys by `tenant:service`.
- Kafka lets consumers lag during spikes without back-pressuring ingestion.
- Processing is at-least-once; ClickHouse rows can duplicate after a crash.
- Rate limits in ingestion are per instance; Kong's are per client IP with the `local` policy.

## Known limits

- Auth is email + password with 12 h HS256 tokens signed by `JWT_SECRET`; no SSO/OIDC, MFA, password reset or email verification yet.
- One tenant per user; roles are stored but not yet enforced per endpoint.
- The query and analysis services connect to ClickHouse with a read/write user; use a read-only user in production.
- The detector uses fixed thresholds; there is no learned anomaly detection.
- Metrics are stored and queryable by API but the dashboard has no metrics page yet.
- Billing and usage metering are not implemented.
