# Tasks

## 1. MCP stdio command

- [x] 1.1 Add and pin the Go MCP SDK dependency; verify the module resolves and `go test ./...` passes.
- [x] 1.2 Add `datawark mcp stdio --config <path>` that bootstraps Runtime before entering the stdio server loop; verify CLI tests cover valid invocation wiring and fail-closed bootstrap errors.
- [x] 1.3 Ensure command diagnostics use stderr and stdout remains reserved for MCP protocol traffic; verify a process-level test observes no non-protocol stdout output.

## 2. Policy-enforced MCP tools

- [x] 2.1 Register only `list_allowed_schema`, `query_data`, and `explain_query`; verify a tool-list test checks exact names and rejects absent administrative/raw-SQL tools.
- [x] 2.2 Implement `list_allowed_schema` using Runtime visibility operations; verify tool tests omit ungranted datasource, relation, and column metadata.
- [x] 2.3 Implement `query_data` with the canonical Query Plan schema and labelled tabular result; verify allowed execution uses Runtime and aliases, raw SQL, principal fields, and unknown fields are rejected before execution.
- [x] 2.4 Implement `explain_query` with canonical plan input and safe decision output; verify denied and invalid plans execute no SQL and disclose no hidden metadata.

## 3. End-to-end verification

- [x] 3.1 Add an in-process MCP stdio protocol test covering initialization, tool discovery, an allowed query, schema visibility, malformed input, and a denied plan; verify protocol framing remains valid.
- [x] 3.2 Add Docker-backed MCP integration coverage using the PostgreSQL fixture; verify `make test-integration` proves an MCP query is bounded by the shared runtime pipeline.
- [x] 3.3 Run `gofmt`, `go vet ./...`, `go test ./...`, and `make test-integration`; verify all checks pass.

## Workflow follow-up

- Apply and archive `public-query-plan-contract` and `gateway-runtime-services` first.
- Archive this change after review before planning MCP HTTP or REST.
