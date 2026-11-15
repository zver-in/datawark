# Tasks

## 1. Structured operational logging

- [ ] 1.1 Construct a process-wide JSON `slog` logger on stderr with stable safe field names; verify lifecycle logs never write to stdout or contain configuration and environment secrets.
- [ ] 1.2 Add HTTP response capture and one completion log per request with request ID, transport, supported operation, outcome, status class, byte count, and duration; verify hostile paths and values are replaced by bounded categories rather than logged.
- [ ] 1.3 Add startup, readiness-withdrawal, and shutdown lifecycle logs; verify ordering and secret redaction in server lifecycle tests.

## 2. Bounded Prometheus metrics

- [ ] 2.1 Add a dedicated Prometheus registry with request-total and duration collectors using fixed transport, operation, authentication, policy, execution, and status-class labels; verify collector tests reject or collapse unknown values to bounded enums.
- [ ] 2.2 Mount unauthenticated `GET /metrics` through the operational allowlist; verify Prometheus parsing succeeds without credentials and output contains no principal, request ID, resource, error text, or other data-plane metadata.
- [ ] 2.3 Instrument authentication and stable runtime outcome categories without importing Prometheus into policy, compiler, schema, or PostgreSQL packages; verify metrics cover success, unauthenticated, deny, timeout, capacity, row-limit, result-size, unavailable, and internal outcomes.

## 3. Cardinality and audit isolation

- [ ] 3.1 Add regression tests that submit many principal, request, source, table, column, path, and error values; verify metric series cardinality remains bounded by documented enum combinations.
- [ ] 3.2 Inject failing logger and metric recorders alongside a successful audit recorder; verify telemetry failure neither changes audit delivery nor duplicates an audit event or successful request outcome.
- [ ] 3.3 Run unit and Docker-backed REST/MCP HTTP traffic tests and scrape metrics; verify `go test ./...` and `make test-integration` pass with stdout reserved for audit and stderr for operational logs.

## Workflow follow-up

- Apply and archive `http-server-foundation` first.
- Complete instrumentation after `http-api-key-authentication`, `rest-data-api`, and `mcp-streamable-http` expose their stable operation and outcome categories.
