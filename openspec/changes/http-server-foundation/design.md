# Design

## Context

The CLI currently bootstraps a runtime only for MCP stdio. There is no HTTP package, listener configuration, signal-aware shutdown, or request correlation. Configuration parsing is strict, so HTTP fields must be added deliberately before transports can mount routes.

## Goals / Non-Goals

**Goals:**

- Own one HTTP listener and lifecycle for all future network transports.
- Refuse to bind until the immutable Gateway runtime is complete.
- Provide deterministic probes, correlation, and shutdown behavior.

**Non-Goals:**

- Expose data routes, authenticate clients, or emit audit and metrics.
- Add TLS termination or dynamic listener reconfiguration.
- Probe PostgreSQL on every readiness request.

## Decisions

### Use `net/http` with a chi router

Create an internal HTTP server package around `net/http.Server` and `chi/v5`. Chi provides composable middleware and route mounting for later REST and MCP handlers without owning lifecycle or security decisions. A separate HTTP process or framework is unnecessary.

### Add a minimal listener configuration

Add optional `server.http.listenAddress`, defaulting to `127.0.0.1:8080`. Binding loopback by default avoids accidental network exposure before API-key configuration exists. Header, idle, maximum-header, and shutdown limits use documented safe constants rather than expanding the v1alpha1 configuration surface prematurely.

### Bootstrap before listener creation

`datawark serve --config` loads configuration, constructs the complete runtime, and only then calls `net.Listen`. Startup errors are normalized before reaching CLI stderr. The server receives constructed dependencies rather than opening PostgreSQL or parsing configuration inside handlers.

### Model readiness as process state

Health reports that the HTTP process is alive. Readiness becomes true immediately before serving after successful bootstrap and false before graceful shutdown begins. Because runtime snapshots are immutable and bootstrap is all-or-nothing, probes do not reconnect to PostgreSQL or mutate readiness per request.

### Correlate every HTTP request at the outer boundary

Use `X-Request-ID`. Accept only 1-128 ASCII letters, digits, `.`, `_`, or `-`; otherwise generate a random 128-bit hexadecimal value. Store the effective ID in request context and response headers without reflecting rejected input.

### Use signal-aware graceful shutdown

The `serve` command derives a cancellation context from interrupt and termination signals. Shutdown first withdraws readiness, then calls `http.Server.Shutdown` with a fixed grace deadline. Header and idle timeouts plus `MaxHeaderBytes` bound stalled clients; query duration remains bounded by existing execution timeouts.

## Risks / Trade-offs

- [A loopback default surprises container users] → Require an explicit non-loopback address in deployment configuration and document it in M5.
- [Readiness does not continuously test PostgreSQL] → Preserve immutable fail-closed bootstrap semantics; add dynamic dependency health only if runtime lifecycle later changes.
- [A fixed shutdown grace can end long requests] → Choose a grace period above default statement timeout and cancel remaining work through request contexts.

## Migration Plan

1. Extend strict configuration parsing with the optional HTTP block and default.
2. Add router, probes, request-ID middleware, and lifecycle tests using ephemeral listeners.
3. Add `serve` with signal-aware shutdown and bootstrap error normalization.
4. Leave only probes mounted until later transport changes apply.

Rollback removes `serve` and the optional HTTP block; existing configurations remain valid throughout.
