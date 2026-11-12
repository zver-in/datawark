# Tasks

## 1. Runtime configuration

- [ ] 1.1 Extend the strict v1alpha1 configuration model and unknown-field checks with optional `server.runtime` limits and documented defaults; verify existing configuration fixtures remain valid and default values are populated.
- [ ] 1.2 Validate positive timeout, row, byte, and concurrency values with field-path diagnostics; verify table-driven tests reject zero, negative, malformed, and unknown runtime fields without leaking secrets.
- [ ] 1.3 Update the redacted example configuration with optional runtime settings; verify `datawark config validate` accepts the example without a DSN value.

## 2. Execution limits

- [ ] 2.1 Add an executor-owned non-blocking concurrency gate and stable capacity failure; verify unit tests reject excess requests before transaction creation and reuse a slot after failure.
- [ ] 2.2 Enforce plan row limits before execution and bounded result-byte collection while scanning; verify unit tests discard partial rows and return stable row/size failures.
- [ ] 2.3 Apply matching context and PostgreSQL local statement deadlines in each read-only transaction; verify executor tests cover timeout/cancellation cleanup.

## 3. PostgreSQL security regression tests

- [ ] 3.1 Add Docker integration tests for a sleeping statement timeout, capacity exhaustion, excessive plan limit, and oversized result; verify each produces its stable failure with no partial result.
- [ ] 3.2 Verify integration tests can execute a permitted subsequent query after every guardrail failure; run `make test-integration` successfully.
- [ ] 3.3 Run `gofmt`, `go vet ./...`, `go test ./...`, and `make test-integration`; verify all checks pass.

## Workflow follow-up

- Apply and archive `read-only-query-execution` first, then archive this change after review.
