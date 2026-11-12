# Spec Delta

## Purpose

Provides one immutable, fail-closed application runtime that safely exposes the completed PostgreSQL core to future transports without giving them direct infrastructure access.

## ADDED Requirements

### Requirement: Bootstrap a complete immutable Gateway runtime
The system SHALL load a validated configuration and metadata snapshot for every configured datasource before making runtime operations available, and SHALL not expose a partially initialized runtime.

#### Scenario: Start with all configured datasource snapshots
- **WHEN** configuration and metadata loading succeed for every configured datasource
- **THEN** the runtime exposes operations using one immutable configuration and its matching snapshots

#### Scenario: Refuse an incomplete startup
- **WHEN** configuration validation or metadata loading fails for any configured datasource
- **THEN** runtime creation fails without exposing query, schema, or explanation operations

### Requirement: Execute a public plan only through the security pipeline
The runtime SHALL execute a public Query Plan only through validation, trusted development-principal authorization, metadata validation, authorized compilation, and read-only execution.

#### Scenario: Execute an allowed runtime query
- **WHEN** a caller submits an allowed valid plan for a loaded datasource
- **THEN** the runtime returns the labelled read-only result from the completed security pipeline

#### Scenario: Reject a denied runtime query
- **WHEN** validation, authorization, metadata validation, compilation, or execution cannot proceed
- **THEN** the runtime returns a safe failure without bypassing an earlier gate

### Requirement: Provide policy-filtered schema and dry-run operations
The runtime SHALL list only datasource and relation metadata visible to its trusted development principal and SHALL explain a plan without executing SQL.

#### Scenario: List visible metadata
- **WHEN** a caller requests schema metadata for a loaded datasource
- **THEN** the runtime returns only the policy-filtered relations and columns for the development principal

#### Scenario: Explain without execution
- **WHEN** a caller requests an explanation for a plan
- **THEN** the runtime returns the safe allow or deny outcome after applicable validation and metadata checks without opening an execution transaction
