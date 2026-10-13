# Resolve-X SDKs

Thin wrappers over OpenTelemetry. One `init()` call turns on traces, metrics and logs, auto-instruments your web framework and database drivers, links every log line to its trace, and can record deployments. The wire format is standard OTLP, so you can leave at any time, and any language with an OpenTelemetry SDK works without one of ours.

| Language | Package | Status |
| --- | --- | --- |
| Node.js ≥ 18 | [`@resolve-x/sdk`](node) | 0.1, tested against a live stack |
| Python ≥ 3.9 | [`resolvex`](python) | 0.1, tested against a live stack |
| Anything else | Standard OpenTelemetry SDK | see "Other languages" |

## Node.js

```bash
npm install @resolve-x/sdk
```

```js
// first line of your entrypoint
const resolvex = require("@resolve-x/sdk");
resolvex.init({ apiKey: process.env.RESOLVEX_API_KEY, serviceName: "checkout", version: process.env.GIT_SHA, markDeployment: true });

resolvex.logger.info("order placed", { orderId: 42 });          // linked to the active trace
await resolvex.withSpan("charge-card", () => charge(card));     // custom span, errors recorded
```

No code change: `RESOLVEX_API_KEY=… RESOLVEX_SERVICE=checkout node -r @resolve-x/sdk/register app.js`

## Python

```bash
pip install "resolvex[web,db]"
```

```python
import resolvex
resolvex.init(api_key="rx_…", service_name="checkout", version=os.environ["GIT_SHA"], mark_deployment_on_start=True)
# import Flask / FastAPI / requests / SQLAlchemy AFTER init()

@resolvex.traced("charge-card")
def charge(card): ...
```

Standard `logging` calls are exported and linked to the current trace.

## Configuration

Every option can also be an environment variable; explicit arguments win.

| Option | Env var | Default |
| --- | --- | --- |
| `apiKey` / `api_key` | `RESOLVEX_API_KEY` | required |
| `serviceName` / `service_name` | `RESOLVEX_SERVICE` or `OTEL_SERVICE_NAME` | required |
| `environment` | `RESOLVEX_ENVIRONMENT` | `NODE_ENV` / `development` |
| `version` | `RESOLVEX_VERSION` | none (enables deployment tracking) |
| `endpoint` | `RESOLVEX_ENDPOINT` | `http://localhost:4318` |
| `apiUrl` / `api_url` | `RESOLVEX_API_URL` | `http://localhost:8000` |
| `autoInstrument` | `RESOLVEX_AUTO_INSTRUMENT` | `true` |

The defaults point at a local stack. There is no hosted Resolve-X service yet, so set `endpoint` and `apiUrl` to your own deployment.

## Other languages

```bash
export OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318
export OTEL_EXPORTER_OTLP_HEADERS="x-resolvex-key=rx_…"
export OTEL_SERVICE_NAME=checkout
export OTEL_RESOURCE_ATTRIBUTES="service.version=1.4.2,deployment.environment=production"
```
Use the official OpenTelemetry SDK or auto-instrumentation agent for Java, Go, .NET, Ruby, PHP or Rust. Both OTLP/HTTP protobuf and JSON are accepted. Record releases with `POST /api/v1/webhooks/deployments` (see `backend/README.md`).

## Releasing

These packages are not published yet. To publish: `cd sdk/node && npm publish --access public` (after reserving the `@resolve-x` scope) and `cd sdk/python && python -m build && twine upload dist/*`.
