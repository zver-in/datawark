# Tasks

## 1. Process-level `serve` stream evidence

- [x] 1.1 Add a bounded child-test-process harness for `serve` by reusing the existing CLI subprocess pattern; verify the parent waits for readiness, sends an authenticated data-plane request, terminates gracefully, drains both OS pipes, and cannot leave a child running on timeout or failure.
- [x] 1.2 Replace the in-process stream-ownership claim with structural assertions over captured process output; verify every stdout line is a complete audit JSON event, every stderr line is operational JSON, startup/request/readiness-withdrawal/shutdown appear in order, expected authentication and terminal-operation events exist, and distinctive config, environment, credential, request, and error secrets appear in neither stream by running `go test ./internal/cli`.

## 2. Complete low-cardinality coverage

- [x] 2.1 Expand the HTTP observability fixture with a supported/fallback request matrix that varies principals, request IDs, API-key values, sources, tables, columns, raw paths, path parameters, status/error-shaped text, and arbitrary headers, queries, and bodies; verify every prohibited dimension crosses an HTTP ownership boundary under a stable bounded outcome.
- [x] 2.2 Prime and parse Prometheus exposition before and after repeating the matrix with many distinctive values; verify request-total and duration families retain identical label-set collections, expose exactly the six owned labels with fixed-enum values, and contain none of the submitted values by running `go test ./internal/httpserver`.

## 3. Docker REST/MCP metrics and sink isolation

- [x] 3.1 Add a build-tagged subprocess scenario that runs the real `datawark serve` command against the existing Docker PostgreSQL fixture and generated config, then performs authenticated MCP Streamable HTTP tool calls, an authenticated REST query, and an unauthenticated `/metrics` scrape before graceful shutdown; verify the scenario completes under explicit readiness and process deadlines with `make test-integration`.
- [x] 3.2 Parse both owned metric families from the Docker scenario and verify the exact expected REST and MCP transport/operation/authentication/policy/execution/status-class combinations, fixed-enum labels for any protocol/probe series, and absence of the API key, principal, request ID, datasource, relation, column, fixture data, DSN, and config values.
- [x] 3.3 Decode the Docker child stdout and stderr independently after shutdown; verify stdout contains only the expected authentication and terminal REST/MCP audit events, stderr contains only correlated operational completion and ordered lifecycle JSON, and neither sink contains records owned by the other.

## 4. Cross-suite verification

- [x] 4.1 Format changed Go test files and run `go vet ./...`, `go test ./...`, `make test-integration`, and `openspec validate --all --strict`; verify all commands pass, `git diff --check` is clean, accepted files under `openspec/specs/` are unchanged, and no production behavior file was modified.

## Workflow follow-up

- Review and archive this verification-only Change after every acceptance assertion and verification command passes.
