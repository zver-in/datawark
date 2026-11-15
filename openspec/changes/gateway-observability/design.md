# Design

## Context

The HTTP foundation supplies request IDs and lifecycle boundaries. REST and MCP HTTP add stable operation and error categories, while audit owns security records and stdout in the `serve` process. Operational telemetry must not leak data, increase metric cardinality, or corrupt MCP stdio protocol output.

## Goals / Non-Goals

**Goals:**

- Give operators correlated request logs and bounded service metrics.
- Keep telemetry failures independent from authorization and audit delivery.
- Preserve stdout exclusively for audit in `serve` and MCP protocol in stdio mode.

**Non-Goals:**

- Distributed tracing, dashboards, alerts, long-term storage, or audit search.
- Per-principal, per-resource, or arbitrary-path telemetry.

## Decisions

### Write operational JSON logs to stderr with `slog`

Construct a process logger using `log/slog` JSON output on stderr. HTTP middleware emits one completion record after response status and byte count are known; lifecycle code emits startup, readiness withdrawal, and shutdown records. Lower layers return safe typed categories rather than logging independently.

This prevents duplicate logs and leaves stdout available for audit JSON lines. MCP stdio protocol output remains untouched.

### Use Prometheus client metrics with fixed labels

Use the Prometheus Go client and register DataWark-owned collectors in a dedicated registry. Core metrics include request totals and duration histograms. Labels are fixed enums: transport, operation, authentication outcome, policy outcome, execution category, and status class. Unknown values collapse to a bounded `other` category.

### Mount a dedicated unauthenticated metrics handler

Expose `/metrics` through the shared router's operational allowlist. It publishes only the dedicated registry and selected safe Go/process collectors. No request ID, principal, resource, raw path, error string, or request value becomes a label or sample.

### Instrument at ownership boundaries

Outer HTTP middleware measures request duration and status. Authentication records its stable outcome. REST and MCP adapters annotate the supported operation and safe policy/execution category in request context for final collection. PostgreSQL and policy packages do not import Prometheus.

### Make telemetry best-effort and audit-independent

Logging and metric recording do not determine request success. Recorder failures are surfaced through a safe internal fallback on stderr where possible but do not create audit events or satisfy audit delivery.

## Risks / Trade-offs

- [Unauthenticated metrics reveal traffic volume] → Expose no principals or resources, retain loopback-safe default binding, and document network restriction in M5.
- [Context annotations can be forgotten] → Provide bounded defaults and route-level tests for each supported operation and outcome.
- [Prometheus dependency increases binary size] → Prefer the standard ecosystem contract selected by ARCH-001 over a custom text encoder.

## Migration Plan

1. Apply HTTP foundation and establish stable transport operation categories.
2. Add logger construction, response capture, and safe structured fields.
3. Add a dedicated Prometheus registry and `/metrics` handler.
4. Instrument authentication, REST, MCP HTTP, and execution outcome mappings.
5. Add cardinality and secret-redaction regression tests.

Rollback removes middleware and `/metrics`; audit and request processing remain unchanged.
