# Tasks

## 1. Query Plan domain boundary

- [x] 1.1 Add transport-neutral Query Plan, scalar, filter, ordering, and validated-plan types under `internal/query`; verify `go test ./internal/query/...` compiles.
- [x] 1.2 Implement field-path validation for required source, exactly two table parts, unique non-empty projection, supported directions, and positive integer limits; verify table-driven unit tests cover every invalid field category.
- [x] 1.3 Implement closed filter operators and scalar/null compatibility checks; verify unit tests cover every accepted operator and each rejected value shape or null comparison.

## 2. Fail-closed validation tests

- [x] 2.1 Add strict input decoding or equivalent boundary checks that reject unknown fields, raw SQL, nested expressions, joins, grouping, and multiple tables without reflecting hostile values; verify negative tests assert sanitized diagnostics.
- [x] 2.2 Add fuzz tests for Query Plan validation and run a bounded `go test -fuzz` invocation to verify malformed input cannot panic or produce a validated unsupported construct.
- [x] 2.3 Run `gofmt`, `go vet ./...`, and `go test ./...`; verify all checks pass without a PostgreSQL connection.

## Workflow follow-up

- Apply and archive this change before implementing `policy-query-authorization`.
