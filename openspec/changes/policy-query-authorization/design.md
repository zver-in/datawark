# Design

## Context

The existing configuration model already carries policies, principals, datasources, and select-only allow entries, but no code consumes them for a query decision. This change follows the validated plan boundary and precedes SQL compilation or database access.

## Goals / Non-Goals

**Goals:**

- Provide a transport-neutral, deterministic allow/deny decision.
- Enforce one-grant complete coverage and deny by default.
- Produce safe explain data for a future trusted administrative adapter.

**Non-Goals:**

- Authenticate callers or accept principals from client input.
- Execute SQL, introspect schema, implement row policies, masking, or hot reload.
- Provide a remote policy administration API.

## Decisions

### Evaluate only a validated plan and trusted principal

The evaluator accepts the validated plan from `query-plan-validation`, the server-derived principal, and an immutable validated configuration snapshot. A Query Plan has no principal field. This preserves the PRD boundary that client content cannot assign privileges.

### Collect all referenced columns before matching one grant

The evaluator builds the union of projection, filter, and ordering columns and compares it against each eligible allow entry. A match requires source, table, `select`, principal, and the whole union in the same entry.

Combining grants would make policy composition surprising and directly contradict the architecture. A set-based comparison also naturally handles duplicated references.

### Model decisions separately from explanations

Return a decision with `allowed`, a stable reason code, and an optional matching policy ID only for allowed results. The explain use case consumes this model without taking a new authorization path. Denials report a coarse reason category so resource existence is not disclosed.

### Fail closed at the configuration boundary

The evaluator receives only a validated snapshot. Missing or unusable configuration yields deny rather than a fallback or partial policy evaluation. Initial configuration reload semantics remain startup-only.

## Risks / Trade-offs

- [No authenticated admin boundary yet] → expose detailed allow information only through a future trusted CLI/admin adapter; this change provides no remote endpoint.
- [Grant search is linear] → acceptable for initial YAML policy sizes; index only after measurement without changing the decision semantics.
- [Policy coverage precedes schema verification] → M2 must require both schema and policy checks before execution.

## Migration Plan

No persisted data changes are required. Rollback removes the evaluator; existing configuration parsing remains compatible because it already enforces select-only grants.
