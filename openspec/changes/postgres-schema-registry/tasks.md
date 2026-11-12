# Tasks

## 1. Schema snapshot domain

- [x] 1.1 Add immutable schema snapshot, relation, and column metadata types bound to one datasource; verify `go test ./internal/schema/...` compiles and covers relation/column lookup.
- [x] 1.2 Implement metadata validation for an already validated Query Plan's source, table, projection, filters, and ordering; verify unit tests accept existing references and reject mismatched source, missing table, and missing columns before compilation.

## 2. PostgreSQL metadata adapter

- [x] 2.1 Add a PostgreSQL catalog reader that resolves the named datasource DSN only through its configured environment variable and returns sanitized failures; verify unit tests cover unknown datasource, unset DSN, and non-secret error output.
- [x] 2.2 Implement one read-only catalog query that returns ordinary, partitioned, view, and materialized-view relation columns with type names and nullability while excluding `pg_catalog` and `information_schema`; verify adapter tests cover filtering and metadata mapping.
- [x] 2.3 Add Docker-backed integration fixtures that create a user relation and view, then verify snapshot contents without reading application rows; run `make test-integration` successfully.

## 3. Policy-filtered schema visibility

- [x] 3.1 Implement visible-schema derivation from a trusted principal, validated configuration snapshot, and schema snapshot; verify unit tests return only the intersection of existing relations/columns and select-only grants.
- [x] 3.2 Add negative visibility tests for ungranted principals, ungranted relations, stale grants, unavailable snapshots, and source mismatches; verify no hidden object name occurs in the returned model or error.
- [x] 3.3 Run `gofmt`, `go vet ./...`, `go test ./...`, and `make test-integration`; verify all checks pass.

## Workflow follow-up

- Archive the change after implementation and review, before starting read-only query execution.
