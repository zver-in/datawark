# Proposal

## Why

Remote MCP clients need the same policy-enforced tools already available locally over stdio. M4 must expose them over the shared authenticated HTTP listener without creating a second security or query path.

## What Changes

- Add an authenticated MCP Streamable HTTP endpoint at `/mcp`.
- Expose exactly `list_allowed_schema`, `query_data`, and `explain_query` with the existing canonical tool schemas.
- Bind each HTTP request or session to the API-key-derived trusted principal and reject identity selection in tool input.
- Reuse the same runtime services and safe result mappings as MCP stdio.
- Integrate request correlation and terminal audit events while keeping protocol output free of diagnostics and secrets.
- Exclude MCP administration tools, raw SQL, a separate listener, and unauthenticated remote sessions.

## Capabilities

### New Capabilities

- `mcp-streamable-http`: Authenticated remote MCP access through the shared Gateway HTTP and security runtime.

### Modified Capabilities

- None.

## Impact

This extends the MCP adapter and HTTP router and may use additional server transport support from the existing MCP SDK. It depends on `trusted-principal-runtime`, `http-server-foundation`, `http-api-key-authentication`, and `gateway-audit-events`, but not on the REST adapter.
