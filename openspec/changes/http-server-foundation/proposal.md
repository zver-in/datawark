# Proposal

## Why

DataWark has a local CLI and MCP stdio server but no managed network service. REST and MCP Streamable HTTP need one fail-closed HTTP lifecycle with explicit operational probes before any data endpoint is exposed.

## What Changes

- Add `datawark serve --config <path>` to bootstrap the complete Gateway runtime before binding a listener.
- Add strict HTTP listen configuration with a loopback-safe default.
- Serve unauthenticated liveness and readiness probes with defined startup and shutdown behavior.
- Generate or accept a safe request ID at the HTTP boundary and return it in responses.
- Add bounded HTTP server timeouts, graceful shutdown, and secret-safe startup failures.
- Do not add REST data operations, MCP HTTP, API keys, audit events, or metrics in this change.

## Capabilities

### New Capabilities

- `http-server-lifecycle`: HTTP process lifecycle, request correlation, operational probes, and safe shutdown behavior.

### Modified Capabilities

- `gateway-configuration`: Accept and validate the supported HTTP listener configuration and defaults.

## Impact

This adds an HTTP adapter and `serve` CLI path around the existing immutable runtime. It changes the strict YAML contract and prepares one listener for later REST, MCP HTTP, authentication, audit, and observability changes.
