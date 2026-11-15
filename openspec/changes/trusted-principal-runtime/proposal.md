# Proposal

## Why

The current Gateway runtime always authorizes with `server.developmentPrincipal`, so a future authenticated HTTP transport cannot safely bind each request to its own server-verified identity. M4 needs a transport-neutral trusted-principal boundary before API keys or network data endpoints are added.

## What Changes

- **BREAKING**: Require runtime query, schema, and explain operations to receive a trusted principal established outside untrusted request payloads.
- Carry that principal through validation, policy evaluation, schema visibility, compilation, and execution without reading identity from a Query Plan.
- Preserve MCP stdio behavior by binding its session to the configured development principal at the local transport boundary.
- Add regression coverage proving one request cannot select or override another principal.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `gateway-runtime-services`: Runtime operations become principal-aware and require a server-trusted identity instead of reading the development principal internally.
- `read-only-query-execution`: The complete execution pipeline authorizes with the trusted principal supplied by its caller.

## Impact

This changes the internal runtime and query-service APIs, their tests, and the MCP stdio adapter. It is the prerequisite for `http-api-key-authentication`, `rest-data-api`, `mcp-streamable-http`, audit attribution, and the CLI explain adapter.
