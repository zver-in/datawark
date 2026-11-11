# Design

## Context

The repository has a strict `v1alpha1` configuration boundary but no query domain package. ARCH-001 requires all future transports to share a limited single-table Query Plan before policy evaluation. Database schema introspection is explicitly deferred to M2.

## Goals / Non-Goals

**Goals:**

- Establish one transport-neutral Go representation for the supported Query Plan.
- Make invalid syntax and unsupported value shapes fail before policy evaluation.
- Preserve only validated data for downstream authorization.

**Non-Goals:**

- Verify a table or column against PostgreSQL metadata.
- Add a REST, MCP, or CLI query command.
- Compile SQL, evaluate policy, or impose a maximum row count.

## Decisions

### Use a typed domain model and a validation boundary

Create an `internal/query` package with explicit source, qualified table, projection, filter, order, limit, operator, direction, and scalar-value types. A validator returns a separate validated representation or field-path violations.

This prevents later layers from handling partially valid plans. Passing an untyped map through policy and compiler would duplicate parsing rules and expand the attack surface.

### Keep validation structural until schema registry exists

The validator checks grammar, identifier shape, uniqueness, scalar kinds, and compatible null operators, but not PostgreSQL column types or object existence. Those need a versioned schema snapshot and belong to M2.

Rejecting plans merely because metadata is unavailable would make this security core depend on live DB I/O; accepting all grammar without later schema validation would be unsafe, so M2 must add that check before execution.

### Treat identifiers and values differently

Identifiers are non-empty syntactic components used later only after authorization. Values remain typed scalar data and are never converted to SQL here. This maintains the later compiler's ability to parameterize values.

### Reuse safe validation diagnostics

Use the existing configuration style: field paths and fixed messages, not reflected submitted values. It gives transports machine-actionable failures without leaking hostile payloads.

## Risks / Trade-offs

- [No database type validation in M1] → M2 schema validation is a mandatory pre-execution step; this change has no executor.
- [A fixed initial operator set can be restrictive] → extend it through a future versioned Query Plan change, not ad hoc parser branches.
- [Different transports could decode JSON differently] → transports must decode into the same domain boundary when they are introduced.

## Migration Plan

This introduces an internal capability with no deployed query transport. Rollback is removal of the unused package and tests; no data or configuration migration is required.
