# gateway-observability Specification

## Purpose

Provides secret-safe operational logs and low-cardinality metrics for Gateway traffic, authorization outcomes, failures, and latency without using security audit events as telemetry.

## Requirements

### Requirement: Emit correlated structured operational logs
The system SHALL emit structured JSON operational logs for HTTP request completion and server lifecycle events using stable field names and the effective request ID, transport, operation, outcome category, status, and duration.

#### Scenario: Log a completed data request
- **WHEN** an authenticated REST or MCP HTTP operation completes
- **THEN** one completion log correlates the request with its transport, operation, safe outcome, status, and duration

#### Scenario: Log a server lifecycle event
- **WHEN** HTTP startup, readiness withdrawal, or shutdown occurs
- **THEN** the server emits a structured lifecycle log without configuration secrets

### Requirement: Exclude sensitive and high-cardinality log content
Operational logs SHALL NOT contain credentials, authorization headers, API key derivatives, DSNs, SQL, bound arguments, rows, request bodies, hidden resource names, raw database errors, or other unbounded user-controlled values.

#### Scenario: Log a hostile failure safely
- **WHEN** a request containing distinctive untrusted values fails
- **THEN** the structured log contains only stable safe categories and does not reflect those values

### Requirement: Record bounded Gateway metrics
The system SHALL record request counts and durations with fixed labels for transport, supported operation, authentication outcome, policy outcome, and stable execution category.

#### Scenario: Observe a denied query
- **WHEN** a query is denied by policy
- **THEN** request count and duration metrics identify the transport, query operation, authenticated state, and deny outcome using bounded labels

#### Scenario: Observe an execution guardrail
- **WHEN** execution ends with timeout, capacity, row-limit, or result-size failure
- **THEN** metrics increment the applicable stable execution category

### Requirement: Enforce low-cardinality telemetry labels
Metrics SHALL NOT use principal, request ID, API key, datasource, table, column, status text, error text, path parameters, or arbitrary user-controlled values as labels.

#### Scenario: Process requests for many resources
- **WHEN** requests use many principals, sources, tables, columns, or request IDs
- **THEN** the number of metric label combinations remains bounded by the documented fixed enums

### Requirement: Publish Prometheus metrics safely
The server SHALL expose Prometheus text metrics at unauthenticated `GET /metrics` and SHALL publish only the bounded Gateway metrics defined by this capability plus safe runtime/process metrics.

#### Scenario: Scrape metrics without credentials
- **WHEN** an operator requests `/metrics` without API-key authentication
- **THEN** the server returns Prometheus-compatible metrics without principal or protected data-plane metadata

### Requirement: Keep audit and telemetry independent
Operational logging or metrics failure SHALL NOT be represented as successful audit delivery, and audit events SHALL NOT be counted on the assumption that an operational log was emitted.

#### Scenario: Operational logger fails
- **WHEN** an operational logger or metrics recorder fails while audit delivery succeeds
- **THEN** the audit outcome remains independent and the telemetry failure does not create or duplicate an audit event
