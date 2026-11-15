# Spec Delta

## Purpose

Provides one safely managed HTTP listener for Gateway transports, including complete runtime bootstrap, operational probes, request correlation, bounded connections, and graceful shutdown.

## ADDED Requirements

### Requirement: Start HTTP only after complete Gateway bootstrap
The system SHALL provide `datawark serve --config <path>` and SHALL bind its configured HTTP listener only after configuration and the complete Gateway runtime have bootstrapped successfully.

#### Scenario: Serve after successful bootstrap
- **WHEN** an operator starts `datawark serve` with a bootstrap-ready configuration
- **THEN** the command binds the configured listener and begins serving HTTP

#### Scenario: Refuse a failed bootstrap
- **WHEN** configuration or runtime bootstrap fails
- **THEN** the command exits non-zero without binding the HTTP listener and without exposing secrets

### Requirement: Expose liveness and readiness probes
The server SHALL expose unauthenticated `GET /api/v1/health` and `GET /api/v1/ready` probes; liveness SHALL report a running process, while readiness SHALL report success only while the complete runtime is available for new requests.

#### Scenario: Report a ready server
- **WHEN** the listener is serving with a complete runtime and is not shutting down
- **THEN** both health and readiness return successful responses

#### Scenario: Withdraw readiness during shutdown
- **WHEN** graceful shutdown begins
- **THEN** readiness returns an unavailable response while liveness remains available until the listener stops

### Requirement: Correlate HTTP requests safely
The server SHALL assign every HTTP request a bounded opaque request identifier, SHALL accept a client-provided identifier only when it matches the supported safe syntax and length, and SHALL return the effective identifier in the response.

#### Scenario: Generate a missing request identifier
- **WHEN** a request omits the supported request ID header
- **THEN** the server generates an opaque identifier and returns it in the response header

#### Scenario: Replace an unsafe request identifier
- **WHEN** a request supplies an oversized or malformed request ID
- **THEN** the server replaces it without reflecting the untrusted value

### Requirement: Bound and gracefully stop HTTP service
The server SHALL enforce finite header and idle timeouts, SHALL stop accepting new requests on shutdown, and SHALL allow in-flight requests a finite grace period before forcefully closing remaining connections.

#### Scenario: Complete an in-flight request during shutdown
- **WHEN** shutdown begins while a request can finish within the grace period
- **THEN** the server lets that request finish and then closes its connection

#### Scenario: Bound a stalled client
- **WHEN** a client does not complete request headers within the configured server deadline
- **THEN** the server closes the request without invoking a Gateway data operation
