# Spec Delta

## Purpose

Authenticates HTTP data-plane clients with statically configured API key secrets and derives a trusted policy principal without allowing request content to select identity.

## ADDED Requirements

### Requirement: Resolve every API key before serving
The system SHALL resolve each configured `keyEnv` during HTTP bootstrap and SHALL refuse to serve when a key is missing, empty, or has the same secret value as another configured key.

#### Scenario: Bootstrap distinct configured keys
- **WHEN** every configured API key environment variable contains a distinct non-empty secret
- **THEN** the authentication boundary becomes available without exposing those values

#### Scenario: Reject incomplete key bootstrap
- **WHEN** a configured secret is absent, empty, or duplicates another configured secret
- **THEN** HTTP bootstrap fails before binding a listener and reports only a safe configuration category

### Requirement: Authenticate protected HTTP requests
The system SHALL require exactly one syntactically valid Bearer credential on every protected HTTP request and SHALL compare it against configured secrets without data-dependent string comparison.

#### Scenario: Accept a valid Bearer credential
- **WHEN** a protected request supplies exactly one Bearer credential matching a configured key
- **THEN** authentication succeeds and produces the principal mapped to that key

#### Scenario: Reject an invalid credential
- **WHEN** a protected request omits credentials or supplies a malformed, unknown, or multiple authorization credential
- **THEN** the server returns a stable unauthorized response without invoking a Gateway data operation

### Requirement: Keep identity server-controlled
The system SHALL attach only the principal derived from the authenticated API key to downstream operations and SHALL NOT accept a principal or role from request paths, query parameters, bodies, MCP arguments, or headers other than the authorization credential.

#### Scenario: Ignore a client-selected principal
- **WHEN** an authenticated request includes content naming a different principal or role
- **THEN** downstream authorization uses only the principal mapped to the API key

### Requirement: Exempt only declared operational endpoints
The system SHALL permit liveness, readiness, and `/metrics` without authentication and SHALL require authentication for every data-plane and unknown HTTP route.

#### Scenario: Call an operational endpoint anonymously
- **WHEN** a caller requests supported health, readiness, or metrics without credentials
- **THEN** the server evaluates that operational endpoint without creating an authenticated principal

#### Scenario: Call a protected or unknown route anonymously
- **WHEN** a caller requests any other route without valid credentials
- **THEN** the server returns an unauthorized response before route-specific processing

### Requirement: Keep API key diagnostics secret-safe
The system SHALL NOT include API key values, derived fingerprints, authorization headers, or secret-bearing comparison details in responses, logs, audit events, or configuration errors.

#### Scenario: Reject a credential without disclosure
- **WHEN** authentication fails for a credential containing a distinctive secret value
- **THEN** no response or diagnostic output contains that value or a reusable derivative
