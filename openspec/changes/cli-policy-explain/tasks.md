# Tasks

## 1. Plan input and output contract

- [x] 1.1 Add a bounded plan-source reader for a file or `--plan -` stdin and decode with `query.Decode`; verify tests reject missing, unreadable, oversized, malformed, trailing, aliased, and unknown-field input without reflection.
- [x] 1.2 Define the versioned JSON explanation DTO with allow, stable reason, and allowed-only policy ID; verify JSON snapshots cover allow and deny without hidden resource details.

## 2. CLI explanation command

- [x] 2.1 Add `policy explain --config <path> --plan <path|->` to the Cobra tree and bind it to the runtime's configured local principal; verify no principal/role flag exists and identity-shaped plan content is rejected by canonical decoding.
- [x] 2.2 Delegate to the principal-bound runtime explain operation and treat completed allow or deny as exit zero; verify configuration, bootstrap, input, and output failures exit non-zero with safe stderr diagnostics.
- [x] 2.3 Preserve stdout as exactly one JSON result and keep diagnostics on stderr; verify command tests with credential-bearing environment and hostile plan values contain no DSN, secret, SQL, hidden name, or submitted value.

## 3. No-execution verification

- [x] 3.1 Add unit tests with recording compiler/executor boundaries; verify allowed, denied, invalid, and metadata-missing explanations never compile or execute SQL.
- [x] 3.2 Add a PostgreSQL-backed integration test for file and stdin explanations; verify expected allow and deny output while an execution counter remains zero and `make test-integration` passes.

## Workflow follow-up

- Apply and archive `trusted-principal-runtime` first.
- This change is independent of the HTTP critical path and may be implemented in parallel afterward.
