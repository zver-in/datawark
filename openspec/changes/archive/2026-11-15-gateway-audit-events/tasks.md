# Tasks

## 1. Audit contract and writer

- [x] 1.1 Define the v1alpha1 audit event enums and typed fields for HTTP authentication and HTTP schema, explain, and query operations; verify schema tests reject invalid combinations and JSON snapshots contain only documented fields.
- [x] 1.2 Implement a concurrency-safe JSON-lines recorder over an injected writer; verify concurrent tests produce one complete line per event with deterministic field encoding and no interleaving.
- [x] 1.3 Add hostile-value redaction tests covering API keys, digests, authorization headers, DSNs, SQL, arguments, rows, bodies, hidden resources, and raw errors; verify none appear in serialized events or recorder failures.

## 2. Terminal event orchestration

- [x] 2.1 Add safe outcome classification for authentication, policy, validation, metadata, guardrail, execution, and unavailable failures; verify every existing stable failure maps to a bounded audit reason without raw error text.
- [x] 2.2 Implement one-owner authentication finalization that emits exactly one event for success or failure; verify audit write failure changes the HTTP outcome to unavailable and never duplicates an event.
- [x] 2.3 Implement a reusable audited-operation wrapper for HTTP schema, explain, and query handlers; verify success is not committed before handoff, failure outcomes are recorded once, and successful data is discarded on writer failure.

## 3. Process integration

- [x] 3.1 Wire `datawark serve` to a stdout JSON-lines audit recorder while keeping operational diagnostics on stderr; verify process tests separate both streams.
- [x] 3.2 Prove MCP stdio remains outside the HTTP audit boundary; verify its stdout contains only MCP protocol output under successful and failing tool calls.
- [x] 3.3 Add HTTP integration tests for authentication and fake data operations, writer failure, and request correlation; verify `go test ./...` passes with one authentication event plus one operation event for an accepted request.

## Workflow follow-up

- Apply and archive `trusted-principal-runtime`, `http-server-foundation`, and `http-api-key-authentication` first.
- Archive this change before `rest-data-api` or `mcp-streamable-http`.
