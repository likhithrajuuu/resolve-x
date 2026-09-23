# Resolve-X — Product Vision

## 1. Vision Statement

Resolve-X is intended to provide engineers with an intelligent operational layer for understanding distributed applications.

The platform should continuously build an understanding of:

* What services exist.
* Where those services run.
* How services communicate.
* Which databases and infrastructure they depend on.
* What the system normally looks like.
* What changed before an incident.
* What evidence is associated with an incident.

When abnormal behavior occurs, Resolve-X should transform this information into an actionable incident investigation.

---

## 2. The Problem We Are Solving

Distributed systems produce enormous amounts of operational data.

A single user request can pass through:

```text
API Gateway
    ↓
Authentication
    ↓
Order Service
    ↓
Inventory Service
    ↓
Payment Service
    ↓
Database
    ↓
External API
```

A failure anywhere along this path can produce symptoms somewhere else.

For example:

```text
Payment Service
      ↓
Database connection pool exhausted
      ↓
Request latency increases
      ↓
HTTP 500 increases
      ↓
Order Service failures increase
      ↓
Users cannot place orders
```

The observable symptoms may therefore be different from the underlying cause.

Resolve-X exists to reduce the amount of manual investigation required to connect these signals.

---

## 3. Product Objective

The primary objective is:

> **Reduce the time and effort required for engineers to understand and respond to production incidents.**

Resolve-X will achieve this through five major capabilities:

```text
Discover
   ↓
Observe
   ↓
Correlate
   ↓
Understand
   ↓
Resolve
```

### Discover

Automatically identify services, infrastructure and dependencies.

### Observe

Collect logs, metrics, traces and operational events.

### Correlate

Connect related signals into a unified incident context.

### Understand

Use deterministic analysis and AI-assisted reasoning to identify probable causes.

### Resolve

Provide engineers with actionable remediation recommendations and, where explicitly authorized, support automated remediation.

---

## 4. Target Users

### Software Engineers

Need to understand failures in services they own.

### Backend Engineers

Need visibility into APIs, databases, caches, message brokers and downstream services.

### SRE / Platform Engineers

Need system-wide visibility, incident correlation and service dependency information.

### Engineering Leads

Need historical incident information, recurring failure patterns and operational insights.

---

## 5. Product Principles

### 5.1 Plug and Play

A customer should be able to connect an existing application with minimal changes.

The preferred onboarding flow is:

```text
Install
   ↓
Authenticate
   ↓
Connect telemetry
   ↓
Automatic discovery
   ↓
Dashboard
```

### 5.2 Language Independent

Resolve-X should not be architecturally tied to Java.

Java, Python, Go and Node.js applications should ultimately produce a common telemetry representation.

### 5.3 Evidence Driven

The system should distinguish between:

```text
Observed evidence
        ↓
Correlation
        ↓
Inference
```

AI conclusions should be based on collected evidence.

### 5.4 Explainable

An incident diagnosis should explain why it was generated.

For example:

```text
Probable Cause:
Database connection pool exhaustion

Evidence:
1. DB latency increased from 120ms to 1.8s.
2. Connection pool exhaustion errors appeared.
3. Payment request latency increased simultaneously.
4. The affected service was deployed 8 minutes earlier.
```

### 5.5 Human Controlled

Resolve-X may recommend remediation, but destructive or high-impact actions should require explicit authorization unless an administrator has deliberately configured automated remediation.

---

## 6. Long-Term Product Direction

The long-term platform should support:

```text
Application Discovery
        ↓
Telemetry
        ↓
Anomaly Detection
        ↓
Incident Creation
        ↓
Root Cause Analysis
        ↓
Historical Comparison
        ↓
Remediation Recommendation
        ↓
Optional Automated Remediation
        ↓
Post-Incident Analysis
```

The system should eventually learn from historical incidents while keeping evidence, tenant isolation and human control as core requirements.

---

## 7. Definition of Success

The first meaningful milestone is not the number of microservices or technologies used.

The first meaningful milestone is:

> A developer can connect a real application to Resolve-X, and Resolve-X can automatically discover the application, receive telemetry, identify its dependencies and produce useful incident context without requiring manual telemetry management.
