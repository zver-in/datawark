# Proposal

## Why

An authenticated network Gateway needs a security record of access attempts and outcomes that is distinct from operational logs. The first audit boundary must be explicit about safe fields and delivery guarantees without adding internal persistence.

## What Changes

- Define a versioned JSON audit event for HTTP authentication and HTTP data-plane operations.
- Record request correlation, transport, operation, trusted principal when known, outcome, safe reason category, and timing without data or secrets.
- Emit one terminal event synchronously before returning a successful data-plane response.
- Treat audit write failure as unavailable and do not return a successful response when its terminal event cannot be handed to the configured writer.
- Use process stdout as the initial sink boundary; durability, rotation, and onward delivery remain responsibilities of the external runtime or collector.
- Exclude API keys, DSNs, SQL, bound values, returned rows, hidden object names, internal queues, retries, a metadata database, and a transactional outbox.

## Capabilities

### New Capabilities

- `gateway-audit-events`: Versioned secret-safe access events and their synchronous handoff guarantees.

### Modified Capabilities

- None.

## Impact

This adds a reusable audit service and HTTP hooks at authentication and runtime operation boundaries. It depends on trusted principals, HTTP request IDs, and API-key outcomes, and will be consumed by REST and MCP HTTP adapters. MCP stdio remains unchanged because its stdout is reserved exclusively for protocol messages.
