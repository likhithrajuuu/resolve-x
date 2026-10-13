# Resolve-X backend

Go services, one binary per service under `cmd/`. Public traffic enters through Kong (`:8000`); OTLP goes straight to ingestion.

| Service | Port | Role |
| --- | --- | --- |
| Kong (OSS) | 8000 | Public API: routing, rate limiting, CORS, request IDs (`deploy/kong/kong.yml`) |
| `ingestion` | 4317 gRPC, 4318 HTTP | OTLP in, API-key auth, per-tenant rate limit, publish to `telemetry.raw` |
| `processor` | – | `telemetry.raw` → per-signal topics + ClickHouse |
| `tenancy` | 8080 (internal) | Signup/login (JWT), team members and roles, invite/reset links, projects, API keys, dashboards, audit log |
| `query` | 8081 (internal) | Services, dependency graph, traces, logs, metrics, usage (ClickHouse, read-only user) |
| `incident` | 8082 (internal) | Incidents, timeline, alert rules, SLOs, notifications (Slack/webhook), webhooks, auto-detector, outbox relay |
| `analysis` | – | Correlates evidence and finds the probable root cause (heuristics, optionally refined by the organization's LLM) |
| `assistant` | 8083 (internal) | AI chat over your telemetry; per-organization AI provider settings |

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

See "AI assistant" below for model providers.

## AI assistant

Each organization picks its own model in **Settings → AI assistant**: Anthropic Claude, OpenAI (Responses API, which also serves the Codex models), Google Gemini, or any OpenAI-compatible server (Ollama, vLLM, LM Studio, OpenRouter, Azure, LiteLLM). The key is encrypted with AES-256-GCM (`SECRETS_KEY`) and never returned by the API. Server-wide defaults can be set with `ANTHROPIC_API_KEY`, `OPENAI_API_KEY` or `GEMINI_API_KEY` (`AI_PROVIDER`, `AI_MODEL` to choose).

The chat is a tool-using agent. Its tools are **read-only GETs against Resolve-X's own APIs, made with the asking user's token**, so it can see what that user can see and nothing more, and it has no way to change anything. Log text is untrusted data and the system prompt says so.

- Telemetry the assistant reads (including log text) is sent to the provider you chose. Incident analysis does the same when a provider is configured.
- A custom base URL is checked against private/loopback/metadata addresses at connection time (SSRF guard). `AI_ALLOW_PRIVATE_BASE_URLS=true` disables that for self-hosted setups; the dev compose file sets it so you can use Ollama on your machine.
- Provider adapters are unit-tested against mock servers written from each vendor's API documentation, not against the live services. If a vendor's API differs, **Test connection** in Settings will say so.

## Team and roles

| | Owner | Admin | Member |
| --- | :-: | :-: | :-: |
| View all telemetry, use the AI assistant, build dashboards | ✓ | ✓ | ✓ |
| API keys, projects, alert rules, SLOs, AI and notification settings | ✓ | ✓ | – |
| Approve or reject remediation | ✓ | ✓ | – |
| Invite members and reset their passwords | ✓ | ✓ (members only) | – |
| Invite admins, change roles | ✓ | – | – |

There is no email delivery yet, so an admin creates an **invite link** (7 days) or **password-reset link** (24 hours) and sends it themselves. Only a hash of each token is stored and the link is shown once. Removing a member, changing a role or resetting a password **revokes that user's existing sessions immediately** (Redis cut-off checked by every service).

## Notifications

Settings → Notifications adds Slack (incoming webhook) or generic webhook channels. They fire when an incident opens, when the probable cause is found, and when it is resolved. Channel URLs are encrypted at rest (Slack URLs contain a secret) and requests go through the SSRF-safe client. Generic webhooks are signed: `X-Resolvex-Signature: sha256=HMAC_SHA256(body, signing_secret)`; the secret is shown once at creation. Deliveries retry on 5xx/429 and are logged.

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

## Production notes

- `deploy/k8s/platform.yaml` holds Deployments, Services, PodDisruptionBudgets, HPAs, Kong and an Ingress for every service; `ingestion.yaml` is the OTLP path. **They parse as valid YAML but have never been applied to a cluster.** Dry-run them with `kubectl apply --dry-run=server` first.
- Services that only read telemetry (query, incident, analysis) connect to ClickHouse as a read-only user (`deploy/clickhouse/init.sql`); only the processor can write or create tables.
- `web/Dockerfile` builds the dashboard (Next.js standalone). `NEXT_PUBLIC_*` URLs are baked in at build time.
- Set strong, unique `JWT_SECRET` and `SECRETS_KEY`; never set `DEV_SEED_USER` or `AI_ALLOW_PRIVATE_BASE_URLS` in production. Changing `SECRETS_KEY` makes stored provider keys and webhook URLs undecryptable.

## Known limits

- Auth is email + password with 12 h HS256 tokens signed by `JWT_SECRET`; no SSO/OIDC, MFA or email verification, and no email delivery (links are shared by hand).
- One tenant per user; a person who works for two organizations needs two logins.
- The detector uses fixed thresholds; there is no learned anomaly detection.
- Usage is reported (Usage page) but there is no billing, quota or plan enforcement.
