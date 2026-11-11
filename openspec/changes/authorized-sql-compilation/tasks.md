# Tasks

## 1. Authorized compilation boundary

- [x] 1.1 Introduce an internal authorized-query representation constructed only from the successful policy path; verify compiler package tests cannot compile a merely public or unvalidated Query Plan.
- [x] 1.2 Define the side-effect-free compiled-statement result with SQL and positional arguments; verify identical authorized input produces identical output.

## 2. PostgreSQL SELECT compiler

- [x] 2.1 Implement fixed-template compilation for projection, two-part table, AND filters, optional ordering, and limit; verify unit tests assert exact SQL and argument order for a representative plan.
- [x] 2.2 Implement PostgreSQL identifier quoting and closed mappings for comparison operators and sort directions; verify unit tests cover quoted identifiers and reject malformed identifier shapes.
- [x] 2.3 Bind every filter value and limit as `$N` arguments; verify injection-shaped strings appear only in arguments and never in generated SQL text.

## 3. Compiler security checks

- [x] 3.1 Add negative and fuzz/property tests for hostile values, invalid authorization output, unsupported clauses, and statement-shape invariants; verify a bounded fuzz run passes without PostgreSQL.
- [x] 3.2 Run `gofmt`, `go vet ./...`, and `go test ./...`; verify the compiler performs no network or database I/O.

## Workflow follow-up

- Apply `query-plan-validation` and `policy-query-authorization` first; archive this change after review.
