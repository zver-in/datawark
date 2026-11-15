# Spec Delta

## Purpose

Provides authenticated remote MCP access over Streamable HTTP while preserving the existing three-tool contract and the shared Gateway authorization and execution pipeline.

## ADDED Requirements

### Requirement: Serve MCP Streamable HTTP on the shared listener
The system SHALL expose the MCP Streamable HTTP protocol at `/mcp` on the Gateway HTTP listener and SHALL NOT start a separate MCP network listener.

#### Scenario: Connect an authenticated remote client
- **WHEN** a client sends a valid MCP Streamable HTTP request to `/mcp` with valid API-key authentication
- **THEN** the shared HTTP server processes it as an MCP request

#### Scenario: Reject an unauthenticated MCP request
- **WHEN** a client calls `/mcp` without valid authentication
- **THEN** the request is rejected before MCP protocol or tool processing

### Requirement: Bind MCP activity to the authenticated principal
The MCP HTTP boundary SHALL bind each request and any resulting session state to the API-key-derived trusted principal and SHALL NOT accept identity selection or identity changes through protocol messages, tool arguments, paths, or query parameters.

#### Scenario: Invoke a tool as the authenticated principal
- **WHEN** an authenticated session calls a supported tool
- **THEN** the shared runtime authorizes that call only as the principal bound at the HTTP boundary

#### Scenario: Attempt to override session identity
- **WHEN** a protocol message or tool argument names a different principal or role
- **THEN** the server ignores it as identity and retains the authenticated principal

### Requirement: Preserve the canonical MCP tool surface
The MCP HTTP server SHALL expose exactly `list_allowed_schema`, `query_data`, and `explain_query` with the same canonical input and output semantics as MCP stdio.

#### Scenario: Enumerate remote tools
- **WHEN** an authenticated MCP HTTP client lists tools
- **THEN** it receives exactly the three supported data tools and their canonical schemas

#### Scenario: Execute a permitted remote query
- **WHEN** an authenticated client calls `query_data` with a permitted canonical Query Plan
- **THEN** the server returns the labelled result from the shared runtime

### Requirement: Preserve safe protocol and audit behavior
MCP HTTP responses SHALL contain only protocol output, SHALL use stable non-sensitive tool failures, and SHALL correlate each accepted operation with the HTTP request ID and its terminal audit event.

#### Scenario: Handle a denied tool call
- **WHEN** a tool call is invalid, unauthorized, or unavailable
- **THEN** the server returns a safe MCP failure without credentials, hidden resources, SQL, arguments, or raw diagnostics

#### Scenario: Fail an unaudited successful call
- **WHEN** a successful tool result cannot be handed to the audit writer
- **THEN** the server returns a safe unavailable tool failure instead of the data result

### Requirement: Match shared transport decisions
For the same trusted principal and canonical Query Plan, MCP HTTP SHALL produce the same authorization decision and runtime outcome category as MCP stdio and REST.

#### Scenario: Compare MCP transports
- **WHEN** MCP HTTP and MCP stdio submit the same plan under equivalent trusted principals and runtime snapshots
- **THEN** both transports report equivalent allow, deny, or failure categories
