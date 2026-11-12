# Design

## Context

Read-only execution introduces a safe SQL path but can still consume database and process resources through long-running, high-cardinality, or concurrent permitted queries. The strict configuration currently contains no runtime resource controls.

## Goals / Non-Goals

**Goals:**

- Add compatible, validated runtime limits with secure defaults.
- Enforce each limit at the narrowest practical point in execution.
- Give callers stable, non-sensitive failure categories and release resources reliably.

**Non-Goals:**

- Implement tenant quotas, distributed rate limiting, request prioritization, or adaptive throttling.
- Change policy grants, Query Plan grammar, or database roles.
- Return partial query results.

## Decisions

### Add optional server runtime settings with defaults

Extend `server` with an optional `runtime` mapping containing duration, row, byte, and concurrency limits. Existing configurations retain compatibility by receiving defaults: 5 seconds, 1,000 rows, 4 MiB, and 16 executions. Explicit values must be positive and unknown nested fields remain invalid.

Defaults make the security property active without forcing an immediate configuration migration; optional configuration allows operators to tune it for a datasource.

### Enforce timeout in PostgreSQL and through context

Apply `SET LOCAL statement_timeout` in the execution transaction and align the Go context deadline with the same limit. PostgreSQL terminates work at the database while context handles connection-side cancellation.

### Reject rather than silently truncate

Validate the plan limit against max rows before executing. While scanning, count encoded result bytes and abort if the threshold would be exceeded; discard accumulated rows before returning the result-size failure. This avoids a misleading partial answer and makes limits observable to a future transport.

### Use a bounded non-blocking execution gate

An executor-owned semaphore is acquired before transaction setup and released with `defer` on all paths. If no slot is immediately available, return capacity exhaustion instead of queueing unbounded work.

## Risks / Trade-offs

- [Result-size accounting depends on eventual serialization] → use a deterministic internal representation now and revisit transport-specific wire sizes later.
- [A 5-second default can reject legitimate analytics] → operators can raise it explicitly; the secure default avoids unlimited work.
- [Per-instance concurrency is not a cluster-wide quota] → add distributed control only with multi-instance deployment.

## Migration Plan

Existing v1alpha1 YAML remains valid and gets defaults. Rollback removes optional runtime fields and executor wrapping; operators relying on custom limits must remove the fields before rolling back to a prior binary.
