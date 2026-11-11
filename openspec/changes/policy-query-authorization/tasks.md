# Tasks

## 1. Authorization decision model

- [ ] 1.1 Consume the validated Query Plan from `query-plan-validation` and add policy decision, reason-code, and safe explanation types; verify `go test ./internal/policy/...` compiles.
- [ ] 1.2 Implement evaluator input using a trusted principal and one validated configuration snapshot, with deny-by-default for missing or unusable input; verify unit tests cover no principal match and unavailable configuration.

## 2. Grant coverage enforcement

- [ ] 2.1 Implement matching of principal, source, table, and select operation against configured policies; verify tests cover a fully allowed query and inaccessible source/table.
- [ ] 2.2 Collect projection, filter, and ordering columns and require their complete coverage by one allow entry; verify tests reject filter-only and sort-only leakage plus grants split across entries.
- [ ] 2.3 Implement dry-run explanation output that exposes a matching policy only for allow decisions and non-disclosing reason codes for denials; verify tests assert prohibited resource names are absent from deny output.

## 3. Security regression checks

- [ ] 3.1 Add table-driven regression cases using the existing configuration fixtures or focused policy fixtures; verify no denied case reaches an authorized result.
- [ ] 3.2 Run `gofmt`, `go vet ./...`, and `go test ./...`; verify policy evaluation requires neither SQL compilation nor a PostgreSQL connection.

## Workflow follow-up

- Apply `query-plan-validation` first, then archive this change before implementing `authorized-sql-compilation`.
