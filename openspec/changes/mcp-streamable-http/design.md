# Design

## Context

The installed MCP Go SDK already provides `mcp.NewStreamableHTTPHandler`. The current MCP package builds three tools over a small Gateway interface and runs them through stdio. HTTP authentication is request-scoped, while stateful MCP sessions could otherwise outlive or be reused with a different credential.

## Goals / Non-Goals

**Goals:**

- Reuse the exact existing tool definitions and runtime facade over `/mcp`.
- Bind every tool call to the principal authenticated for its HTTP request.
- Avoid identity drift and session persistence in the first remote MCP release.

**Non-Goals:**

- Server-to-client requests, resumable events, long-lived session state, custom OAuth, or a second listener.
- New tools or transport-specific authorization logic.

## Decisions

### Use the SDK Streamable HTTP handler in stateless mode

Mount `mcp.NewStreamableHTTPHandler` with `Stateless: true` at `/mcp`. The server factory obtains the trusted principal from request context, binds the runtime facade, and creates the same three-tool MCP server used by stdio.

Stateless mode is chosen over retained sessions because the current tools need no server-side session data and each HTTP request can be bound independently to its authenticated principal. It also avoids credential changes on a reused session ID and the SDK's session-retention lifecycle.

### Share tool registration, not transport instances

Refactor MCP construction so a bound Gateway facade produces one canonical server definition. Stdio supplies the local bound facade; HTTP supplies a request-bound facade. Tool names, schemas, result DTOs, and safe error mappings remain common.

### Authenticate outside the MCP handler

API-key and request-ID middleware wrap `/mcp` before the SDK parses protocol messages. The server factory returns no server without a trusted identity. Identity fields in JSON-RPC or tool arguments have no effect.

### Audit tool operations at the Gateway adapter

Wrap the bound Gateway passed to MCP HTTP so each schema, query, or explain invocation owns one terminal audit event. Authentication middleware records its separate event. Tool success is returned only after audit handoff; failures remain stable MCP errors.

### Preserve SDK security defaults

Keep localhost/DNS-rebinding protection enabled, set a bounded SDK logger directed to safe operational logging, and use the shared HTTP shutdown context. Do not enable an event store or standalone SSE behavior.

## Risks / Trade-offs

- [Some MCP clients expect stateful sessions or server notifications] → Declare stateless Streamable HTTP for M4 and cover supported clients in interoperability tests.
- [Creating a server per request adds allocation] → The three-tool server is small; measure before introducing session caches that complicate identity.
- [SDK protocol errors may contain unexpected diagnostics] → Wrap tool failures, configure safe logging, and add secret-shaped regression inputs.

## Migration Plan

1. Apply trusted principal, HTTP foundation, API-key authentication, and audit changes.
2. Extract shared MCP tool registration over a bound Gateway facade.
3. Mount the stateless Streamable HTTP handler at `/mcp` behind HTTP middleware.
4. Add SDK client interoperability, auth isolation, audit, and parity tests.

Rollback unmounts `/mcp`; MCP stdio continues using the same shared tool registration.
