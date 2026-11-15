# Spec Delta

## Purpose

Provides authenticated versioned REST operations for policy-filtered schema discovery, bounded query execution, and dry-run explanation through the shared Gateway security runtime.

## ADDED Requirements

### Requirement: Expose authenticated REST data routes
The system SHALL expose `POST /api/v1/query`, `POST /api/v1/query/explain`, `GET /api/v1/schema`, and `GET /api/v1/schema/{source}` on the shared HTTP listener and SHALL require an authenticated trusted principal for each route.

#### Scenario: Execute an authenticated query
- **WHEN** an authenticated caller posts a permitted canonical Query Plan to `/api/v1/query`
- **THEN** the server returns the labelled result produced by the shared Gateway runtime for that principal

#### Scenario: Reject an unauthenticated data request
- **WHEN** a caller invokes any REST data route without valid authentication
- **THEN** the server rejects it before decoding a plan or invoking a runtime operation

### Requirement: Preserve the canonical Query Plan contract
Query and explain routes SHALL accept exactly one bounded JSON document using the canonical Query Plan fields and SHALL reject unknown fields, legacy aliases, trailing documents, unsupported content types, and request bodies larger than the configured fixed limit.

#### Scenario: Decode a canonical plan
- **WHEN** a request supplies one supported JSON Query Plan within the body limit
- **THEN** the adapter delegates its validated public plan to the shared runtime

#### Scenario: Reject unsupported request content
- **WHEN** a request contains a legacy alias, unknown field, trailing JSON value, unsupported media type, or oversized body
- **THEN** the adapter returns a stable invalid-request response without policy evaluation or execution

### Requirement: Return only policy-visible schema
The schema collection route SHALL return all datasources with metadata visible to the authenticated principal, and the source route SHALL return only visible metadata for its named datasource without distinguishing hidden from nonexistent sources.

#### Scenario: List visible schema
- **WHEN** an authenticated caller requests `/api/v1/schema`
- **THEN** the response contains only datasource, relation, and column metadata visible to that principal

#### Scenario: Request an unavailable source
- **WHEN** an authenticated caller requests a source that is hidden, unknown, or unavailable
- **THEN** the server returns the same stable not-found response without disclosing which condition occurred

### Requirement: Explain without execution
The explain route SHALL apply canonical decoding, trusted-principal authorization, and metadata checks and SHALL return a safe allow or deny result without compiling SQL or opening an execution transaction.

#### Scenario: Explain a denied plan
- **WHEN** an authenticated caller posts a valid but unauthorized plan to `/api/v1/query/explain`
- **THEN** the response contains a stable deny reason without hidden resource names and no SQL is executed

### Requirement: Return stable correlated responses
Every REST response SHALL include the effective request ID and SHALL use versioned JSON success or error shapes whose error codes and messages do not expose credentials, SQL, arguments, database diagnostics, policy names on deny, or hidden resources.

#### Scenario: Return a runtime failure safely
- **WHEN** the shared runtime returns a validation, denial, timeout, capacity, result-size, or unavailable failure
- **THEN** the REST adapter maps it to the documented stable status and error code with the effective request ID

### Requirement: Match shared transport decisions
For the same trusted principal and canonical Query Plan, REST SHALL produce the same authorization decision and runtime outcome category as MCP transports.

#### Scenario: Compare REST and MCP decisions
- **WHEN** equivalent REST and MCP calls submit the same plan for the same trusted principal and runtime snapshot
- **THEN** both transports report equivalent allow, deny, or failure categories

### Requirement: Publish the REST contract
The system SHALL provide a versioned OpenAPI document covering every REST route, authentication requirement, canonical Query Plan schema, success shape, error envelope, and request ID header defined by this capability.

#### Scenario: Validate the published contract
- **WHEN** the REST contract or handler schemas change
- **THEN** automated validation detects any mismatch between the OpenAPI document and the supported route shapes
