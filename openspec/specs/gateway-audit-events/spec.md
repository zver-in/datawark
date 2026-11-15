# gateway-audit-events Specification

## Purpose

Produces a versioned, secret-safe security record for HTTP authentication and HTTP data-plane operations with an explicit synchronous handoff boundary to an external collector.

## Requirements

### Requirement: Emit versioned terminal audit events
The system SHALL produce one terminal JSON audit event for each HTTP authentication attempt and each accepted HTTP schema, explain, or query operation, including event version, timestamp, request ID, transport, operation, outcome, safe reason category, duration, and trusted principal when known.

#### Scenario: Record an authenticated query outcome
- **WHEN** an authenticated query operation completes with allow, deny, or error
- **THEN** exactly one terminal event identifies the request, transport, operation, trusted principal, outcome, safe reason category, and duration

#### Scenario: Record failed authentication
- **WHEN** an HTTP authentication attempt fails before a principal is established
- **THEN** exactly one terminal event records the authentication failure without a principal or credential value

#### Scenario: Keep MCP stdio protocol output isolated
- **WHEN** the Gateway runs as an MCP stdio process
- **THEN** this HTTP audit capability writes no event to that process's protocol stdout

### Requirement: Exclude sensitive and untrusted content
Audit events SHALL NOT contain API keys or derivatives, authorization headers, DSNs, SQL text, bound arguments, result rows, request bodies, hidden object names, raw error text, or other user-controlled values outside validated bounded identifiers.

#### Scenario: Audit a hostile request safely
- **WHEN** a rejected request contains secrets, injection-shaped text, or hidden resource names
- **THEN** its audit event contains only the stable safe fields and categories defined by the audit schema

### Requirement: Hand off audit events synchronously
The system SHALL synchronously write a terminal event before returning a successful data-plane response and SHALL return a stable unavailable failure instead of success when that event cannot be handed to the configured writer.

#### Scenario: Return success after audit handoff
- **WHEN** a data-plane operation succeeds and its terminal event is written successfully
- **THEN** the transport may return the successful operation response

#### Scenario: Fail closed on audit writer failure
- **WHEN** a data-plane operation would succeed but its terminal event cannot be written
- **THEN** the transport returns a safe unavailable failure and does not return the data result

### Requirement: Define the stdout delivery boundary
The initial audit sink SHALL write one complete JSON event per line to process stdout without user-space retry or buffering guarantees; persistence, rotation, deduplication, and onward delivery SHALL remain responsibilities of the process runtime or external collector.

#### Scenario: Hand an event to stdout
- **WHEN** the stdout writer accepts the complete serialized event
- **THEN** the Gateway considers its audit delivery obligation satisfied

#### Scenario: Encounter process termination after handoff
- **WHEN** the process or host fails after a successful writer handoff
- **THEN** the Gateway makes no claim of durable or exactly-once storage beyond that boundary
