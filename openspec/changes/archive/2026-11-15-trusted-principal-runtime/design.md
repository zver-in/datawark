# Design

## Context

`runtime.Runtime` owns an immutable validated configuration, schema snapshots, and executor, but its public methods read `Server.DevelopmentPrincipal` internally. `execution.QueryService` repeats that coupling. Policy evaluation already accepts a caller-supplied principal, while MCP stdio currently receives the unbound runtime directly.

## Goals / Non-Goals

**Goals:**

- Make identity binding explicit before any query, schema, or explain operation.
- Give transports a narrow principal-bound service surface rather than configuration or policy access.
- Preserve current MCP stdio behavior and fail-closed ordering.

**Non-Goals:**

- Authenticate HTTP credentials or define API key configuration.
- Change policy matching semantics or expose identity selection publicly.
- Add audit, REST, or MCP HTTP behavior.

## Decisions

### Introduce a trusted principal value and a bound Gateway facade

Add an internal identity value with a validated non-empty principal name. Runtime exposes a binding operation used only by trusted adapters and returns a facade implementing `Execute`, `Datasources`, `Schema`, and `Explain` without further identity arguments. The facade retains the immutable runtime plus exactly one principal.

Binding once is preferred over adding a raw string argument to every tool handler: it prevents accidental mixing of principals within one request or MCP server instance and keeps transport interfaces small. The type is an architectural trust marker, not a cryptographic boundary; constructors remain inside server-side packages and tests cover dependency direction.

### Keep one explicit local binding

Runtime exposes a local binding derived from `server.developmentPrincipal` for MCP stdio and CLI adapters. Network adapters must bind an identity returned by their authenticator and cannot fall back to the local binding.

This preserves compatibility while making the insecure local mode visibly different from authenticated serving.

### Pass the principal into QueryService

`execution.QueryService` gains an explicit trusted principal field and rejects its zero value before policy evaluation. Schema filtering and explain use the same bound value. Configuration continues to carry policies and the local development principal but no longer silently supplies identity to a network-capable operation.

### Adapt MCP stdio through the bound interface

The MCP server's `Gateway` interface stays transport-neutral. The CLI constructs the complete runtime, obtains its local bound facade, and passes that facade to MCP stdio. Tool input schemas remain unchanged and contain no identity field.

## Risks / Trade-offs

- [Internal Go code can still misuse a trust-marker constructor] → Keep constructors in the identity/authentication boundary, pass bound facades to transports, and add tests that public payloads cannot influence the principal.
- [Breaking internal method signatures touch many tests] → Migrate runtime, execution service, MCP adapter, and integration fixtures in one change before parallel M4 work starts.
- [Local mode remains unauthenticated] → Limit its binding to local adapters and never use it as an HTTP fallback.

## Migration Plan

1. Add trusted identity and bound facade types alongside the current runtime.
2. Move QueryService, schema, and explain paths to the explicit principal.
3. Migrate MCP stdio and all tests to the local bound facade.
4. Remove the old implicitly bound runtime methods after repository callers compile against the new boundary.

Rollback restores the internal implicit methods; no persisted data or public wire contract changes.
