# Tasks

## 1. Completion telemetry semantics

- [x] 1.1 Add a bounded derived completion outcome with explicit precedence over authentication, policy, execution, and HTTP status annotations; verify table-driven telemetry tests cover success, unauthenticated, denied, timeout, capacity, row limit, result size, unavailable, internal, and unknown combinations.
- [x] 1.2 Emit the derived `outcome` in the single HTTP completion log and classify matched schema-detail requests as `schema_get`; verify route-level tests assert every required completion field while hostile paths, parameters, headers, bodies, errors, and hidden resource names remain absent.
- [x] 1.3 Strengthen lifecycle and process-stream tests to assert startup, readiness-withdrawal, and shutdown ordering, configuration/environment secret redaction, audit-only stdout, and JSON operational logs on stderr.

## 2. Metrics and isolation verification

- [x] 2.1 Parse unauthenticated `/metrics` output with the Prometheus text parser and verify owned request-total and duration families expose only the fixed transport, operation, authentication, policy, execution, and status-class labels.
- [x] 2.2 Exercise success, unauthenticated, policy denial, timeout, capacity, row-limit, result-size, unavailable, internal, and unknown outcomes through HTTP ownership boundaries; verify each produces the expected bounded metric labels without importing Prometheus into policy, compiler, schema, or PostgreSQL packages.
- [x] 2.3 Expand cardinality regression coverage across many principals, request IDs, sources, tables, columns, paths, and error values; verify the parsed metric model contains only the expected fixed-enum series and no submitted value.
- [x] 2.4 Inject failing operational logger and metric recorders around a successful audited data operation; verify the response remains singular and successful while exactly one authentication event and one terminal operation event are delivered without duplication.

## 3. CLI explanation verification

- [x] 3.1 Add valid-plan configuration/bootstrap and local-binding failure cases for `policy explain`; verify each exits non-zero with empty stdout and safe stderr while malformed input still stops before bootstrap.
- [x] 3.2 Add the narrow internal compiler test seam needed to record compile calls without changing default runtime behavior; verify allowed, denied, invalid, and metadata-missing CLI explanations invoke neither compiler nor executor and retain the existing versioned JSON results.

## 4. End-to-end verification

- [x] 4.1 Extend the Docker-backed PostgreSQL suite with authenticated REST and MCP HTTP traffic followed by an unauthenticated `/metrics` scrape; verify Prometheus parsing, expected bounded series, absence of protected metadata, and unchanged audit/operational stream ownership.
- [x] 4.2 Run `gofmt`, `go vet ./...`, `go test ./...`, `make test-integration`, and `openspec validate --all --strict`; verify every command passes and the accepted specs remain unchanged.

## Workflow follow-up

- Review and archive this corrective Change after all verification evidence passes.
