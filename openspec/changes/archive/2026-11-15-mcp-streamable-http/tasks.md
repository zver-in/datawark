# Tasks

## 1. Shared MCP server construction

- [x] 1.1 Refactor MCP tool registration to construct the canonical three-tool server from any principal-bound Gateway facade; verify existing stdio protocol and tool-schema tests remain unchanged.
- [x] 1.2 Add a request-bound MCP Gateway wrapper that owns terminal audit events for schema, query, and explain calls; verify success waits for audit handoff and failures produce one safe event.

## 2. Streamable HTTP transport

- [x] 2.1 Mount the SDK `StreamableHTTPHandler` in stateless mode at `/mcp` on the shared router; verify it uses no event store, session retention, standalone listener, or disabled localhost protection.
- [x] 2.2 Build each request's MCP server from the API-key-derived principal in context; verify missing identity returns no server and identity fields in JSON-RPC or tool arguments cannot alter the binding.
- [x] 2.3 Configure SDK diagnostics through the safe operational logger and map tool failures to stable non-sensitive MCP results; verify hostile credentials, plans, errors, SQL-shaped text, and hidden names never reach protocol output.

## 3. Protocol and security verification

- [x] 3.1 Add Streamable HTTP client tests for initialization, tool listing, each of the three tools, invalid input, and unauthenticated access; verify exactly the canonical tool surface is available.
- [x] 3.2 Add two-key isolation tests across independent stateless requests; verify each call sees only its authenticated principal's schema and policy outcome even when clients send session or identity-shaped values.
- [x] 3.3 Add audit failure and request-correlation tests; verify successful tool data is withheld on writer failure and protocol responses remain valid.

## 4. Shared runtime integration

- [x] 4.1 Add Docker-backed MCP HTTP query and schema tests against the existing PostgreSQL fixture; verify `make test-integration` passes without affecting MCP stdio.
- [x] 4.2 Add parity tests across MCP HTTP, MCP stdio, and REST where available; verify equivalent identities and plans produce equivalent allow, deny, and failure categories.

## Workflow follow-up

- Apply and archive `trusted-principal-runtime`, `http-server-foundation`, `http-api-key-authentication`, and `gateway-audit-events` first.
- This change may be applied in parallel with `rest-data-api`.
