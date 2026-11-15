# Tasks

## 1. Trusted identity boundary

- [x] 1.1 Add the internal trusted-principal value and principal-bound Gateway facade; verify unit tests reject empty identities and show a bound facade cannot change principal between calls.
- [x] 1.2 Add an explicit local binding derived from `server.developmentPrincipal`; verify tests distinguish local binding from network-supplied trusted identity and expose no identity field to Query Plan decoding.

## 2. Principal-aware security pipeline

- [x] 2.1 Require an explicit trusted principal in `execution.QueryService` and authorize with it; verify missing identity and denied identity stop before compiler and executor invocation.
- [x] 2.2 Move runtime query, schema, datasource listing, and explain operations onto the bound facade; verify two principals over the same immutable runtime receive their own policy decisions and visible metadata.
- [x] 2.3 Preserve explain's no-execution guarantee for the bound principal; verify allowed, denied, invalid, and metadata-missing plans never invoke the executor.

## 3. Local adapter migration

- [x] 3.1 Update MCP server construction to accept a principal-bound Gateway interface; verify its three tool names and public schemas remain unchanged.
- [x] 3.2 Bind MCP stdio to the configured development principal at bootstrap; verify process and in-memory MCP tests cannot select identity through arguments and still return the expected visible schema and query result.
- [x] 3.3 Migrate runtime, execution, schema, and integration fixtures away from implicit principal lookup; verify `go test ./...` and `make test-integration` pass.

## Workflow follow-up

- Apply and archive this change before `http-api-key-authentication`, `gateway-audit-events`, `rest-data-api`, `mcp-streamable-http`, or `cli-policy-explain`.
