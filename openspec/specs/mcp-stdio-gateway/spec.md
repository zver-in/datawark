# mcp-stdio-gateway Specification

## Purpose

Provides a local MCP stdio interface for AI clients to discover allowed PostgreSQL metadata, execute permitted bounded queries, and inspect decisions through the shared Gateway security runtime.

## Requirements

### Requirement: Run a local MCP server over stdio
The system SHALL provide `datawark mcp stdio --config <path>` and SHALL communicate MCP protocol messages only through standard input and standard output without starting a network listener.

#### Scenario: Start a configured MCP stdio server
- **WHEN** an operator invokes the command with a valid bootstrap-ready configuration
- **THEN** the process serves the MCP protocol on stdio and does not bind a network port

#### Scenario: Reject failed bootstrap before serving
- **WHEN** configuration or runtime bootstrap fails
- **THEN** the command exits non-zero without emitting a protocol response that claims the server is ready

### Requirement: Expose only policy-enforced data tools
The MCP server SHALL expose `list_allowed_schema`, `query_data`, and `explain_query`, and SHALL not expose tools for raw SQL, policy administration, datasource administration, or identity selection.

#### Scenario: Enumerate available tools
- **WHEN** an MCP client lists tools
- **THEN** it receives exactly the three supported data tools and their canonical Query Plan schemas where applicable

### Requirement: Route MCP calls through Gateway runtime services
The MCP server SHALL obtain the principal only from the configured Gateway runtime and SHALL delegate schema, query, and explanation calls to its runtime services without direct PostgreSQL, policy, or compiler access.

#### Scenario: Query permitted data
- **WHEN** a client calls `query_data` with an allowed canonical Query Plan
- **THEN** the server returns the runtime's labelled read-only result

#### Scenario: Explain a denied plan
- **WHEN** a client calls `explain_query` with a plan that is denied or invalid
- **THEN** the server returns a safe decision or error without executing SQL or disclosing unavailable metadata

### Requirement: Preserve stdio protocol and safe diagnostics
The MCP server SHALL write only MCP protocol output to stdout and SHALL return stable non-sensitive failures without DSNs, credentials, compiled SQL, arguments, or hidden object names.

#### Scenario: Handle a malformed tool request
- **WHEN** a client submits malformed or unsupported tool input
- **THEN** the server returns a protocol error without reflecting untrusted values to stdout or mixing diagnostics into protocol output
