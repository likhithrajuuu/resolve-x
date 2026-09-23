# ADR-009: Human Approval for Remediation

## Status

Accepted

## Context

Automated remediation can have significant operational consequences.

An incorrect automated action can make an incident worse.

## Decision

Resolve-X will initially provide remediation recommendations rather than automatically executing destructive actions.

Future automated remediation will require:

* Explicit configuration
* Permissions
* Audit logging
* Approval policies
* Rollback mechanisms

## Initial Model

```text
AI Recommendation
       ↓
Engineer Review
       ↓
Approval
       ↓
Execution
       ↓
Result
```
