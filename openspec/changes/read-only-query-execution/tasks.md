# Tasks

## 1. Execution orchestration

- [ ] 1.1 Add a transport-neutral QueryService that invokes Query Plan validation, trusted-principal policy authorization, schema validation, compilation, and execution in order; verify unit tests prove each failed stage prevents executor invocation.
- [ ] 1.2 Define labelled tabular result and safe execution error types, preserving column order and database NULL; verify unit tests cover empty rows, null values, and non-secret failures.

## 2. PostgreSQL read-only executor

- [ ] 2.1 Add datasource resolution and statement execution using the configured DSN environment variable only; verify unit tests cover unknown source, missing DSN, and sanitized connection failures.
- [ ] 2.2 Execute exactly one compiler statement in an explicit read-only transaction and close it on every path; verify adapter tests reject non-compiler input at the boundary.
- [ ] 2.3 Scan PostgreSQL rows into the tabular result without logging SQL or arguments; verify unit tests cover scan and row-close failures.

## 3. Integration and security verification

- [ ] 3.1 Extend Docker integration fixtures with permitted data, a NULL, an ungranted column, and an inaccessible relation; verify an allowed Query Plan returns expected rows while policy and schema failures execute no statement.
- [ ] 3.2 Add integration regression coverage proving the executor cannot perform INSERT, UPDATE, DELETE, or DDL through its public path; verify `make test-integration` passes.
- [ ] 3.3 Run `gofmt`, `go vet ./...`, `go test ./...`, and `make test-integration`; verify all checks pass.

## Workflow follow-up

- Apply and archive `postgres-schema-registry` first, then archive this change before applying runtime guardrails.
