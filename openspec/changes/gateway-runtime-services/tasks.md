# Tasks

## 1. Immutable bootstrap

- [x] 1.1 Add a runtime constructor that loads a validated configuration and creates a metadata snapshot for every configured datasource; verify unit tests cover a complete multi-datasource startup and immutable snapshot ownership.
- [x] 1.2 Make bootstrap fail closed for invalid configuration, missing DSNs, and any catalog-load failure; verify tests expose no partially usable runtime or secret values.

## 2. Transport-neutral operations

- [x] 2.1 Implement runtime query execution that resolves the plan source to its matching snapshot and delegates only through the existing QueryService; verify a fake executor is not called for unknown, denied, or stale metadata plans.
- [x] 2.2 Implement policy-filtered datasource and schema visibility operations; verify unit tests omit ungranted sources, relations, columns, and stale grants.
- [x] 2.3 Implement dry-run explanation through grammar, policy, and metadata checks without execution; verify allowed and denied outcomes and prove the executor receives no call.

## 3. Integration verification

- [x] 3.1 Add Docker-backed runtime bootstrap tests for multiple configured PostgreSQL datasources and one unavailable datasource; verify `make test-integration` proves all-or-nothing startup.
- [x] 3.2 Run `gofmt`, `go vet ./...`, `go test ./...`, and `make test-integration`; verify all checks pass.

## Workflow follow-up

- Apply and archive `public-query-plan-contract` before exposing its plan types through a transport.
- Archive this change after review before applying `mcp-stdio-gateway`.
