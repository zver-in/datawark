# Spec Delta

## Purpose

Provides trusted local operators with a machine-readable dry-run of a canonical Query Plan through validation, policy, and schema checks without compiling or executing SQL.

## ADDED Requirements

### Requirement: Explain a plan from the CLI
The system SHALL provide `datawark policy explain --config <path> --plan <path|->`, where `--plan -` reads one canonical Query Plan JSON document from standard input.

#### Scenario: Explain a plan file
- **WHEN** an operator supplies valid configuration and a readable file containing one canonical Query Plan
- **THEN** the command evaluates the plan and writes one machine-readable explanation

#### Scenario: Explain a plan from stdin
- **WHEN** an operator uses `--plan -` and supplies one canonical Query Plan on standard input
- **THEN** the command evaluates that document without reading another plan source

### Requirement: Use the configured development principal
The command SHALL bind the explanation to `server.developmentPrincipal` and SHALL NOT accept a principal or role from flags, environment variables, or Query Plan content.

#### Scenario: Attempt to select another principal
- **WHEN** plan content or an unsupported argument attempts to name another principal or role
- **THEN** the command does not use it as identity and evaluates only for the configured development principal

### Requirement: Explain without query execution
The command SHALL apply canonical decoding, validation, policy evaluation, and metadata checks and SHALL NOT compile SQL, open an execution transaction, or return query rows.

#### Scenario: Explain an allowed plan
- **WHEN** a plan is valid, allowed for the configured principal, and present in the loaded schema snapshot
- **THEN** the command reports allow with the matching policy identifier and performs no SQL execution

#### Scenario: Explain a denied plan
- **WHEN** a plan is invalid, unauthorized, or absent from visible metadata
- **THEN** the command reports a stable safe deny category and performs no SQL execution

### Requirement: Return stable secret-safe CLI output
The command SHALL write one versioned JSON result to stdout on a completed explanation and SHALL use non-zero exit status with concise stderr diagnostics for input, configuration, bootstrap, or internal failures without exposing secrets or hidden resources.

#### Scenario: Report a completed deny decision
- **WHEN** evaluation completes with a policy or metadata denial
- **THEN** stdout contains a deny result and the command exits successfully because the explanation operation completed

#### Scenario: Reject malformed input
- **WHEN** the plan source contains malformed, oversized, trailing, or unsupported JSON
- **THEN** the command exits non-zero without reflecting submitted values, SQL, DSNs, or environment secrets
