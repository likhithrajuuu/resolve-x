# ADR-005: Service Discovery Strategy

## Status

Accepted

## Context

Resolve-X must automatically discover services without requiring users to manually register every application.

The platform must also support multiple languages and deployment environments.

## Decision

For Kubernetes environments, Resolve-X will combine:

1. OpenTelemetry service metadata
2. Kubernetes metadata
3. Runtime dependency observations
4. Resolve-X Service Registry

The Service Registry stores the result of discovery.

## Important Distinction

Resolve-X is not replacing Kubernetes service discovery.

Kubernetes answers:

```text
Where can service X be reached?
```

Resolve-X answers:

```text
What services exist?
What do they depend on?
What is their health?
What telemetry belongs to them?
```

## Alternatives

### Eureka

Rejected as the primary strategy because it would unnecessarily couple discovery to a Java-oriented application ecosystem.

### Consul

Not selected for the initial Kubernetes architecture because it would introduce another service-discovery control plane.

Consul may be considered later for VM/bare-metal/multi-environment discovery.

### Network scanning

Rejected because it is unreliable, expensive and potentially problematic from a security perspective.

### Manual registration

Rejected as the primary mechanism because it conflicts with the plug-and-play product goal.

## Consequences

The platform becomes strongly integrated with OpenTelemetry and Kubernetes metadata while retaining a provider abstraction for future environments.
