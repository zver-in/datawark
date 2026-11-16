# Design

## Context

See `proposal.md` for motivation. The current lifecycle test replaces the server logger with an in-memory handler, so it proves event content and order but not the `serve` process stream wiring. The current CLI stream test captures Cobra writers while the operational logger writes directly to `os.Stderr`, so it cannot prove what the child process actually emits. Cardinality coverage currently varies only unknown paths, query parameters, and request IDs. The Docker-backed REST/MCP test parses `/metrics`, but checks family presence and label count rather than the traffic-specific series, and it runs behind `httptest.Server` without capturing the CLI audit and operational sinks.

The accepted specifications already define the behavior. This change must add evidence without changing endpoint behavior, label schemas, audit guarantees, or production wiring.

## Goals / Non-Goals

**Goals:**

- Observe `serve` through real OS stdout and stderr pipes and validate each emitted line structurally.
- Prove that every dimension forbidden by the telemetry contract cannot increase owned metric cardinality.
- Prove Docker-backed REST and MCP HTTP requests create the expected bounded series while audit and operational records remain in their respective process streams.
- Keep the new tests deterministic, bounded in time, and independent of exposition ordering or incidental Go/process metrics.

**Non-Goals:**

- Add or rename telemetry, change audit contents, or alter server lifecycle behavior.
- Add production-only observability hooks, global logger replacement, or new external dependencies.
- Assert the values or ordering of runtime/process collectors not owned by DataWark.

## Decisions

### Exercise `serve` in a child test process

Reuse the existing child-test-binary pattern used by MCP stdio tests. A helper branch will run the real `serve` command with OS stdout and stderr, while the parent reserves a loopback address, starts the helper, waits for readiness, sends an authenticated data-plane request, terminates it gracefully, and captures both pipes.

The parent will decode every non-empty stdout line as an audit event and every non-empty stderr line as an operational JSON object. It will assert lifecycle order (`startup`, request completion, `readiness_withdrawn`, `shutdown`), expected audit operations, and absence of distinctive configuration, environment, credential, request, and error secrets from both streams. This observes the actual `audit.NewJSONLinesRecorder(cmd.OutOrStdout())` and `telemetry.NewLogger()` wiring without adding a production seam.

An in-process logger test alone was rejected because it cannot establish process stream ownership. Temporarily replacing global `os.Stdout` or `os.Stderr` was rejected because it is race-prone and would not exercise the command boundary.

### Compare metric label sets before and after hostile-value expansion

Build a table of supported and fallback HTTP requests that covers principals, request IDs, API-key values, datasource names, table names, column names, raw paths and path parameters, error/status-shaped text, and arbitrary header/body/query values. Prime the `/metrics` scrape, record the owned label-set baseline, repeat the same bounded outcomes with many distinctive values, and parse a final scrape with the Prometheus text parser.

The request-total and duration families must expose exactly the owned six label names, every value must belong to the documented fixed enums, and the final label-set collection must equal the baseline collection. The exposition must contain none of the submitted distinctive values. Comparing sets rather than sample counts proves that additional inputs do not create series while avoiding brittle assertions about counter or histogram values.

Substring-only checks were rejected because they do not detect an unexpected but innocuous-looking label. A single unknown-route loop was rejected because it does not drive protected values through supported REST and authenticated boundaries.

### Run Docker evidence through the real CLI process

Add a build-tagged integration subprocess that starts `datawark serve` against the existing Docker PostgreSQL fixture and a generated configuration. The parent will use an authenticated MCP Streamable HTTP client and an authenticated REST query, then scrape `/metrics` without credentials before gracefully stopping the child.

The parsed owned families will be checked for the exact expected REST and MCP transport/operation/authentication/policy/execution/status-class label combinations created by the calls. All other owned series must still use only allowed enum values, accommodating protocol handshake and operational probe requests without depending on their count or order. Distinctive API key, principal, source, relation, column, request ID, SQL fixture data, DSN, and configuration values must be absent from the exposition.

After shutdown, stdout and stderr will be decoded independently: stdout must contain only the expected authentication and terminal-operation audit events, while stderr must contain only operational JSON including the correlated REST/MCP completions and lifecycle events. This combines the real Docker runtime with the real process sink wiring; an injected `httptest.Server` logger seam was rejected because it would verify a test-only construction instead of the deployed CLI boundary.

### Bound subprocess lifecycle explicitly

Both parent tests will use context deadlines, readiness polling with a fixed deadline, graceful interrupt/termination, and forced cleanup on failure. Pipe readers will drain concurrently so verbose failure output cannot block the child. Assertions will parse JSON and Prometheus models rather than depend on line or exposition ordering except where lifecycle ordering is itself the contract.

## Risks / Trade-offs

- [Loopback address selection can race between reservation and child bind] → Keep the reservation window minimal, fail with captured diagnostics, and retry address allocation only inside the bounded test harness.
- [MCP Streamable HTTP may use multiple protocol requests] → Assert exact series for the owned tool outcomes and enum validity for additional handshake series, not a fixed total HTTP request count.
- [Subprocess failures can leave a child running] → Register cleanup immediately after start, use deadlines, and escalate from graceful termination to kill only in test cleanup.
- [Audit and operational schemas may be confused by generic JSON parsing] → Decode stdout into the audit event shape and validate required audit fields; validate stderr against required operational keys and explicitly reject audit-only records.
- [Cardinality tests can miss a forbidden value hidden by encoding] → Use distinctive ASCII tokens, inspect parsed label names and values, and also scan the raw exposition.

## Migration Plan

1. Add the bounded cardinality matrix and parsed label-set comparison to the HTTP observability tests.
2. Replace the in-process `serve` stream claim with a child-process assertion while retaining focused lifecycle unit coverage.
3. Add the Docker-backed CLI subprocess scenario for REST, MCP HTTP, metrics, and sink isolation.
4. Run formatting, vet, unit tests, integration tests, and strict OpenSpec validation.

There is no deployment migration. Rollback removes only the added verification harness and tests.
