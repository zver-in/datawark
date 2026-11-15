# Spec Delta

## ADDED Requirements

### Requirement: Validate HTTP listener configuration
The system SHALL accept an optional `server.http.listenAddress` host-and-port value, SHALL reject malformed or unknown HTTP configuration fields, and SHALL use `127.0.0.1:8080` when the listen address is absent.

#### Scenario: Use the loopback listener default
- **WHEN** an existing valid configuration omits `server.http`
- **THEN** HTTP server configuration resolves the listen address to `127.0.0.1:8080`

#### Scenario: Accept an explicit listener
- **WHEN** `server.http.listenAddress` contains a valid host-and-port value
- **THEN** strict configuration validation retains that address

#### Scenario: Reject malformed HTTP configuration
- **WHEN** the HTTP listen address is malformed or `server.http` contains an unsupported field
- **THEN** validation fails with the applicable field path and without exposing environment values
