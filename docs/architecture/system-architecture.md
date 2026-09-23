# Resolve-X System Architecture

## 1. System Context

Resolve-X sits between customer applications/infrastructure and engineering teams.

```text
┌─────────────────────────────────────────────────────────┐
│                    CUSTOMER SYSTEM                      │
│                                                         │
│  Java     Python      Go       Node.js                  │
│    │         │         │          │                    │
│    └─────────┴─────────┴──────────┘                    │
│                     │                                   │
│               OpenTelemetry                            │
│                     │                                   │
│              Kubernetes / VM                           │
└─────────────────────┼───────────────────────────────────┘
                      │
                      ▼
             ┌─────────────────┐
             │ Resolve-X Agent │
             │ / OTel Collector│
             └────────┬────────┘
                      │ OTLP
                      ▼
             ┌─────────────────┐
             │ Ingestion Layer │
             └────────┬────────┘
                      │
                      ▼
                    Kafka
                      │
       ┌──────────────┼──────────────┐
       ▼              ▼              ▼
     Logs           Metrics         Traces
       │              │              │
       └──────────────┼──────────────┘
                      ▼
             Correlation Engine
                      │
          ┌───────────┴───────────┐
          ▼                       ▼
   Service Registry        Incident Service
          │                       │
          └───────────┬───────────┘
                      ▼
               AI Analysis
                      │
                      ▼
                API Gateway
                      │
                      ▼
                 Dashboard
```

---

# 2. Architectural Layers

Resolve-X is divided into the following logical layers.

```text
┌─────────────────────────────┐
│ Presentation                │
│ Dashboard / API             │
├─────────────────────────────┤
│ Application                 │
│ Incident / Analysis         │
├─────────────────────────────┤
│ Intelligence                │
│ Correlation / Discovery     │
├─────────────────────────────┤
│ Processing                  │
│ Telemetry / Events          │
├─────────────────────────────┤
│ Infrastructure              │
│ Kafka / DB / Redis / K8s    │
└─────────────────────────────┘
```

---

# 3. Core Services

## Ingestion Service

Responsible for accepting telemetry and validating ingestion requests.

Responsibilities:

* Authentication
* Tenant resolution
* Telemetry validation
* Metadata enrichment
* Event publishing

It should not perform expensive root-cause analysis.

---

## Service Registry

Responsible for maintaining Resolve-X's understanding of discovered services.

Responsibilities:

* Service registration
* Service updates
* Service lifecycle
* Metadata
* Dependency relationships

It is an observability catalog, not a replacement for Kubernetes service discovery.

---

## Correlation Service

Responsible for connecting related operational signals.

Responsibilities:

* Time-window correlation
* Service correlation
* Trace correlation
* Deployment correlation
* Dependency correlation
* Evidence generation

---

## Incident Service

Responsible for the incident lifecycle.

Responsibilities:

* Incident creation
* Incident state
* Severity
* Timeline
* Assignment
* Evidence references
* Incident closure

---

## Analysis Service

Responsible for AI-assisted analysis.

Responsibilities:

* Build analysis context
* Call LLM provider
* Parse structured response
* Validate output
* Store analysis
* Generate recommendations

The analysis service must not become a dependency for basic incident creation.

---

## API Gateway

Responsible for external API access.

Responsibilities:

* Authentication
* Authorization
* Routing
* Rate limiting
* Request validation

---

# 4. Service Communication

## REST

Use REST for:

```text
Dashboard → API Gateway
External integrations → Resolve-X
Administrative APIs
Incident queries
Service queries
```

## Kafka

Use Kafka for:

```text
Telemetry processing
Incident events
Service discovery events
Analysis events
Notification events
```

## gRPC

Use gRPC selectively for internal synchronous operations where a strongly typed, low-overhead RPC contract is justified.

gRPC should not be introduced merely for the sake of using it.

---

# 5. Data Stores

## PostgreSQL

Primary transactional data:

```text
tenants
users
projects
environments
services
dependencies
incidents
incident_timeline
analyses
api_keys
```

---

## Redis

Transient/high-speed state:

```text
active incidents
deduplication keys
temporary correlation state
rate limiting
hot cache
distributed locks where required
```

Redis must not become the system of record for critical business data.

---

## Kafka

Event transport:

```text
telemetry.raw
telemetry.logs
telemetry.metrics
telemetry.traces

service.discovered
service.updated

incident.created
incident.updated

analysis.requested
analysis.completed
```

---

## ClickHouse

Long-term analytical telemetry may be stored in ClickHouse when data volume and analytical workloads justify it.

Potential data:

```text
historical logs
telemetry aggregates
incident analytics
service performance
historical dependency observations
```

ClickHouse is not the primary transactional database.

---

# 6. Data Flow

## Telemetry Flow

```text
Application
    │
    ▼
OpenTelemetry SDK / Agent
    │
    ▼
OTLP
    │
    ▼
Resolve-X Collector
    │
    ▼
Ingestion Service
    │
    ▼
Kafka
    │
    ├── Log Processor
    ├── Metric Processor
    └── Trace Processor
```

---

## Service Discovery Flow

```text
Telemetry
    │
    ▼
Extract Resource Attributes
    │
    ├── service.name
    ├── service.version
    ├── environment
    └── runtime metadata
    │
    ▼
Kubernetes Metadata Enrichment
    │
    ▼
Discovery Engine
    │
    ▼
Service Registry
```

---

## Dependency Discovery Flow

```text
Trace
  │
  ├── Service A
  │
  ├── HTTP span
  │
  └── Service B
         │
         └── DB span
               │
               ▼
            PostgreSQL

                ↓

Dependency Engine

Service A → Service B → PostgreSQL
```

---

# 7. Incident Flow

```text
Alert / Anomaly
       │
       ▼
Incident Service
       │
       ▼
Create Incident
       │
       ▼
Define Investigation Window
       │
       ▼
Correlation Engine
       │
       ├── Metrics
       ├── Logs
       ├── Traces
       ├── Dependencies
       └── Deployments
       │
       ▼
Incident Context
       │
       ▼
Analysis Service
       │
       ▼
AI Analysis
       │
       ▼
Incident Updated
```

---

# 8. AI Architecture

AI is deliberately placed after deterministic evidence collection.

```text
                 Raw Telemetry
                      │
                      ▼
                Correlation
                      │
                      ▼
               Evidence Set
                      │
                      ▼
             Incident Context
                      │
                      ▼
                AI Analysis
                      │
             ┌────────┴────────┐
             ▼                 ▼
       Root Cause          Recommendations
```

The AI should not be responsible for:

* Discovering raw services
* Querying arbitrary infrastructure
* Determining whether telemetry exists
* Making unsupported factual claims

The deterministic platform should provide the evidence.

---

# 9. Failure Isolation

A failure in one component should not cascade through the entire platform.

For example:

```text
AI Service DOWN
      │
      X
      │
Incident creation continues
      │
      ▼
Incident available
      │
      ▼
AI analysis marked:
"PENDING"
```

Similarly:

```text
Analytics Storage DOWN
      │
      X
      │
Core incident management continues
```

---

# 10. Deployment Model

Initial development:

```text
Docker Compose
```

Production target:

```text
Kubernetes

├── API Gateway
├── Ingestion
├── Service Registry
├── Incident Service
├── Correlation Service
├── Analysis Service
└── Kafka
```

Stateful infrastructure may initially be externally managed or deployed separately.

---

# Service-Level Architecture

## Ingestion Service

### Responsibilities

* Receive OTLP telemetry
* Authenticate requests
* Resolve tenant
* Validate telemetry
* Enrich metadata
* Publish processing events

### Does not own

* Incident lifecycle
* AI analysis
* Long-term telemetry analytics

---

## Service Registry

### Responsibilities

* Maintain service identity
* Track service versions
* Track environments
* Track first/last seen
* Track service state
* Maintain dependency metadata

### Example entity

```text
Service
---------
id
tenant_id
project_id
environment_id
name
language
framework
version
status
first_seen_at
last_seen_at
```

### Dependency

```text
ServiceDependency
-----------------
id
source_service_id
target_service_id
type
first_seen_at
last_seen_at
request_count
error_count
```

---

## Incident Service

### Responsibilities

* Incident lifecycle
* Severity
* Status
* Timeline
* Assignment
* Evidence references

### Lifecycle

```text
OPEN
  ↓
INVESTIGATING
  ↓
IDENTIFIED
  ↓
MITIGATING
  ↓
RESOLVED
  ↓
CLOSED
```

---

## Correlation Service

### Responsibilities

* Gather related telemetry
* Build incident timeline
* Identify affected dependencies
* Detect temporal relationships
* Generate evidence set

### Correlation inputs

```text
Incident
Logs
Metrics
Traces
Services
Dependencies
Deployments
Infrastructure events
```

---

## Analysis Service

### Responsibilities

```text
Receive incident context
        ↓
Build analysis prompt
        ↓
Call LLM
        ↓
Validate structured output
        ↓
Persist analysis
        ↓
Publish analysis.completed
```

### Example result

```json
{
  "summary": "...",
  "probableCauses": [],
  "evidence": [],
  "recommendations": [],
  "confidence": 0.0
}
```

---

# Related Documents

* [System context](system-context.md)
* [Container diagram](container-diagram.md)
* [Data flow](data-flow.md)
* [Service documents](services/README.md)
* [Architecture decision records](../adr/README.md)
