# Design

## Context

M4 HTTP requests have a request ID and, after authentication, a trusted principal. Audit must record authentication plus accepted HTTP data-plane operations. MCP stdio reserves stdout for protocol messages, so the initial stdout sink applies only to the `serve` process and does not alter stdio behavior.

## Goals / Non-Goals

**Goals:**

- Produce deterministic security events with a small safe schema.
- Prevent a successful HTTP data response when its terminal event cannot be handed off.
- State the exact boundary between Gateway handoff and external durability.

**Non-Goals:**

- Durable storage, retries, buffering, deduplication, signing, encryption, or audit search.
- Replacing operational logs or metrics.
- Auditing MCP stdio or local CLI operations in this milestone.

## Decisions

### Define one v1alpha1 terminal event schema

An event contains `version`, UTC timestamp, request ID, transport (`rest` or `mcp_http`), operation (`authenticate`, `schema_list`, `schema_get`, `query`, or `explain`), principal when established, outcome, stable reason category, and duration milliseconds. Fields are typed enums where possible. Resource names, plans, request values, results, and raw errors are absent.

### Separate authentication and operation events

Every authentication attempt emits its own terminal event. A successfully authenticated data request then emits one operation event. This preserves visibility into rejected credentials and makes operation outcomes attributable without embedding credential details.

### Centralize terminal ownership

Authentication middleware owns the authentication event. Each accepted REST handler or MCP tool wrapper owns exactly one operation event using a single deferred finalizer. Runtime code returns typed safe categories but does not write transport audit events, preventing duplicate records across adapters.

### Serialize complete JSON lines synchronously

Define a narrow `Recorder` interface and a mutex-protected JSON-lines writer. Encoding completes before a single writer handoff, preventing concurrent events from interleaving. The `serve` process directs the recorder to stdout; operational logging uses stderr.

### Fail closed at the response boundary

An audit write failure replaces the pending HTTP or MCP success with a stable unavailable failure. For an authentication or operation failure, an audit write failure also returns unavailable so the caller is not told an unaudited security outcome completed normally. A query may already have read data internally before handoff fails, but its rows are discarded.

The Gateway promises synchronous writer acceptance only. It does not promise that the OS, container runtime, collector, or storage persists the line, and it performs no internal retry.

## Risks / Trade-offs

- [A successful database read followed by audit failure can be retried] → Return no rows, keep operations read-only, include request IDs for external correlation, and document at-least-attempt semantics rather than exactly-once execution.
- [Stdout backpressure increases request latency] → Keep events small and bounded; external operators must drain stdout reliably.
- [Process failure can lose an accepted line downstream] → State the handoff boundary explicitly and defer durability to M5 deployment guidance.
- [Two events per successful request increase volume] → Preserve separate authentication evidence; collectors can correlate them by request ID.

## Migration Plan

1. Apply trusted principal, HTTP foundation, and API-key authentication changes.
2. Add event types, safe classifiers, recorder, and JSON-lines writer with concurrency and redaction tests.
3. Integrate authentication event ownership.
4. Expose operation wrappers for subsequent REST and MCP HTTP changes.
5. Verify `datawark mcp stdio` still writes protocol messages only to stdout.

Rollback removes HTTP audit hooks and returns the `serve` process to having no security event guarantee; no stored schema migration is needed.
