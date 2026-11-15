# Design

## Context

See `proposal.md` for motivation. The accepted observability contract already requires safe operation and outcome categories, bounded metrics, parseable Prometheus output, lifecycle ordering, and audit independence. The current server has mutable request telemetry annotations and a dedicated registry, but completion logs omit a single outcome field, schema-detail routing is classified as `other`, and tests do not cover every required category or integration path. The CLI explanation path is functional, but its tests do not isolate a valid-input bootstrap failure or record both compilation and execution boundaries.

## Goals / Non-Goals

**Goals:**

- Make completion telemetry unambiguously satisfy the existing operation and outcome contract.
- Demonstrate every archived observability acceptance condition with deterministic unit or Docker-backed integration coverage.
- Demonstrate CLI bootstrap safety and the no-compilation/no-execution boundary without changing CLI output.

**Non-Goals:**

- Change accepted requirements, public routes, CLI flags, audit events, metric names, or metric labels.
- Add tracing, dashboards, alerts, new telemetry backends, or new runtime behavior.
- Reopen or edit archived Changes.

## Decisions

### Derive one bounded completion outcome from existing annotations

Add a telemetry helper that derives a stable completion outcome from the existing authentication, policy, execution, and HTTP status categories. Use an explicit precedence so authentication denial, policy denial, execution guardrails, unavailability, internal failure, and success cannot produce ambiguous logs. Emit this value as the completion log's `outcome` field while retaining the existing detailed bounded fields.

This avoids a second mutable outcome channel that could disagree with the labels already used for metrics. Deriving an outcome from raw error text was rejected because it would duplicate adapter classification and risk unbounded values.

### Let matched handlers refine supported operations

Keep the boundary classifier responsible for fixed top-level routes and set `schema_get` from the matched schema-detail handler through the existing mutable telemetry context. Tests will cover both collection and detail routes plus hostile unmatched paths. Raw paths and path parameters never become log fields or metric labels.

Using string interpolation or datasource names in the operation label was rejected because it would violate the low-cardinality contract.

### Test collectors through the Prometheus model

Parse `/metrics` with the Prometheus text parser and assert metric families and label values from the parsed model instead of relying only on substring checks. Add table-driven coverage for success, unauthenticated, policy denial, timeout, capacity, row limit, result size, unavailable, internal, and unknown-value collapse. Cardinality tests will vary every prohibited dimension named by the accepted requirement and assert that only bounded combinations are created.

### Preserve best-effort telemetry and fail-closed audit ownership

Keep logger and metric recorder failures ignored by request outcome while audit delivery remains independently fail-closed. Expand the injected-failure test to cover a successful data operation, assert one authentication event plus one terminal operation event, and prove no duplicate audit or response occurs.

### Use existing test seams and add only a narrow compiler seam if required

Exercise CLI input, bootstrap, output, and executor behavior through existing dependencies. To record compilation without creating an alternate explanation implementation, add only the narrow compiler dependency needed by runtime execution and inject a failing or counting compiler in tests; the explanation path must never invoke it. The default remains the current authorized compiler, so production behavior and public interfaces remain unchanged.

### Add one Docker-backed observability scenario

Extend the existing PostgreSQL integration suite to send authenticated REST and MCP HTTP traffic, scrape `/metrics` without credentials, parse the response, and verify expected bounded series without protected metadata. Stream-separation assertions remain at the process/CLI boundary where stdout and stderr can be observed directly.

## Risks / Trade-offs

- [A derived outcome can drift from detailed labels] → Centralize precedence in one telemetry helper and cover every allowed combination table-wise.
- [Operation annotation can be omitted by a future handler] → Retain the bounded `other` fallback and add route-to-operation regression tests for every current route.
- [A compiler test seam could broaden runtime construction] → Keep it internal, optional, and defaulted to the current compiler; do not expose it through transport contracts.
- [Prometheus integration assertions can become brittle] → Parse metric families and assert only owned names and bounded labels, not exposition ordering or incidental Go/process values.

## Migration Plan

1. Add bounded outcome derivation and schema-detail operation annotation with focused tests.
2. Expand lifecycle, parser, category, cardinality, and telemetry/audit isolation tests.
3. Add the CLI bootstrap and compile/execute boundary tests.
4. Add Docker-backed REST/MCP scrape coverage and process stream assertions.
5. Run formatting, vet, unit tests, integration tests, and strict OpenSpec validation.

Rollback removes the corrective implementation and tests; no persisted data or external contract migration is involved.
