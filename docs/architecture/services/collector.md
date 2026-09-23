# Resolve-X Collector

## Purpose

Connect customer applications to Resolve-X without making them depend on the Resolve-X control plane (ADR-011).

The collector is a distribution and configuration of the OpenTelemetry Collector rather than a custom agent.

---

## Responsibilities

* Receive OTLP from applications (gRPC `4317`, HTTP `4318`).
* Enrich telemetry with Kubernetes metadata.
* Batch and compress telemetry.
* Buffer locally when Resolve-X is unreachable.
* Attach the ingestion API key.
* Optionally filter or redact sensitive attributes before export.

---

## Does Not Own

* Service identity. The collector attaches metadata; the Service Registry decides identity.
* Any analysis or alerting.

---

## Deployment

```text
Kubernetes cluster
│
├── DaemonSet: resolve-x-collector (agent mode)
│      one per node, receives from local pods
│
└── Deployment: resolve-x-gateway-collector (optional)
       central aggregation, single egress point
```

Installed with a Helm chart or manifests (roadmap M1 / M8).

---

## Key Components (OpenTelemetry Collector)

| Component               | Use                                                |
| ----------------------- | -------------------------------------------------- |
| `otlp` receiver         | Receive application telemetry                      |
| `k8sattributes` processor | Add namespace, deployment, pod, node, labels     |
| `resourcedetection` processor | Add cloud and host attributes                |
| `memory_limiter` processor | Protect the node                                |
| `batch` processor       | Efficient export                                   |
| `otlp` exporter with sending queue | Export to Resolve-X with retry and buffering |

---

## Configuration Sketch

```yaml
receivers:
  otlp:
    protocols:
      grpc:
      http:

processors:
  memory_limiter:
    check_interval: 1s
    limit_percentage: 80
  k8sattributes: {}
  batch: {}

exporters:
  otlp/resolvex:
    endpoint: ingest.resolve-x.example:4317
    headers:
      x-resolvex-api-key: ${env:RESOLVEX_API_KEY}
    sending_queue:
      enabled: true
    retry_on_failure:
      enabled: true

service:
  pipelines:
    traces:  { receivers: [otlp], processors: [memory_limiter, k8sattributes, batch], exporters: [otlp/resolvex] }
    metrics: { receivers: [otlp], processors: [memory_limiter, k8sattributes, batch], exporters: [otlp/resolvex] }
    logs:    { receivers: [otlp], processors: [memory_limiter, k8sattributes, batch], exporters: [otlp/resolvex] }
```

The endpoint and header name are placeholders until the ingestion API is implemented.

---

## Failure Behaviour

| Failure                       | Behaviour                                                  |
| ----------------------------- | ---------------------------------------------------------- |
| Resolve-X unreachable         | Queue and retry; drop oldest data when the queue is full.  |
| Collector itself down         | Applications' OTel exporters fail silently; apps continue. |
| Memory pressure               | `memory_limiter` refuses data before the node is at risk.  |

---

## Observability

The collector's own metrics (queue size, dropped items, export failures) are exported to Resolve-X so that a broken collector is itself visible.
