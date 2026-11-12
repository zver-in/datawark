# Design

## Context

M1 supplies a validated Query Plan, opaque policy capability, and parameterized PostgreSQL statement. `postgres-schema-registry` supplies the missing database metadata check. The current adapter only pings and cannot execute statements.

## Goals / Non-Goals

**Goals:**

- Establish one application-level path from a public plan to a read-only result.
- Enforce validation, policy, schema, compilation, and execution in that order.
- Keep transport-independent rows and safe errors for later REST/MCP adapters.

**Non-Goals:**

- Add a network endpoint, identity provider, schema refresh, or observability pipeline.
- Add writes, arbitrary SQL, or query result resource limits.
- Infer policy permissions from PostgreSQL privileges.

## Decisions

### Make QueryService the sole execution orchestrator

The service accepts an unvalidated plan and trusted runtime/configuration dependencies, then calls query validation, policy authorization, schema validation, compiler, and executor in order. The executor accepts only the compiler statement and is not exposed to transports.

This prevents a future adapter from accidentally skipping an enforcement stage. Individual packages remain separately testable using fakes.

### Use an explicit PostgreSQL read-only transaction

The PostgreSQL executor resolves the selected configured datasource, starts one read-only transaction, executes the statement once with arguments, scans rows, and closes the transaction on every path. The explicit transaction is defense in depth alongside the restricted statement compiler and DB role.

### Return a neutral tabular result

Represent results as ordered column names plus ordered row values, retaining nil for database NULL. Avoid a JSON-shaped representation until an API transport defines its contract. Execution failures use stable internal categories; adapter/driver detail is logged only through a future observability boundary.

### Treat metadata and policy as independent gates

The service evaluates policy before metadata and compilation. A policy denial therefore does not turn schema lookup into a discovery oracle, while metadata still prevents a stale policy grant from becoming executable.

## Risks / Trade-offs

- [No resource bounds in this change] → `postgres-runtime-guardrails` must follow before any network transport is introduced.
- [Driver result values are database-specific] → preserve native values internally; transport serialization belongs in the API milestone.
- [A broad DB role could still read more than policy permits] → policy/schema/compiler remain mandatory; deployments use least-privilege roles.

## Migration Plan

No migration or config syntax change is required. Deploy after `postgres-schema-registry`; rollback removes the QueryService and executor while retaining the preceding security layers.
