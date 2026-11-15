# Proposal

## Why

Operators need to distinguish traffic, denials, runtime failures, and latency without inspecting audit records or sensitive request content. M4 therefore needs a bounded operational telemetry contract separate from the security audit trail.

## What Changes

- Add structured operational logs correlated by request ID.
- Add bounded counters and duration metrics for HTTP transport, operation, authentication outcome, policy denial, and execution failure category.
- Publish Prometheus metrics on an unauthenticated operational endpoint without exposing data-plane labels.
- Define a low-cardinality label policy that excludes principals, request IDs, datasource/table/column names, error text, and user-controlled values.
- Keep audit event delivery, traces, alerting, dashboards, and external telemetry storage outside this change.

## Capabilities

### New Capabilities

- `gateway-observability`: Secret-safe structured operational logging and low-cardinality Gateway metrics.

### Modified Capabilities

- None.

## Impact

This adds telemetry interfaces and an HTTP metrics handler around the M4 server and data transports. It depends on `http-server-foundation` and is completed after REST and MCP HTTP operation names and outcomes are available.
