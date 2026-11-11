# Spec Delta

## Purpose

Определяет единое, deny-by-default решение о доступе к валидному Query Plan на основе декларативных select-only политик Gateway.

## ADDED Requirements

### Requirement: Authorize a validated Query Plan against the trusted principal
The system SHALL evaluate a validated Query Plan using a principal supplied by the trusted server-side identity boundary and SHALL NOT read a principal or role from the Query Plan itself.

#### Scenario: Allow a configured principal
- **WHEN** the trusted principal is listed in a policy whose grant covers the query
- **THEN** the system returns an allow decision for that plan

#### Scenario: Deny an unconfigured principal
- **WHEN** the trusted principal is absent from every applicable policy
- **THEN** the system returns a deny decision without executing SQL

### Requirement: Require complete coverage by one allow entry
The system SHALL allow a Query Plan only when one allow entry under an applicable policy covers its source, qualified table, `select` operation, projection columns, filter columns, and ordering columns; permissions from different entries SHALL NOT be combined.

#### Scenario: Allow a fully covered plan
- **WHEN** one grant covers the plan's source, table, and every referenced column
- **THEN** the system returns an allow decision

#### Scenario: Deny a partially covered plan
- **WHEN** projection, filtering, or ordering needs columns split across two allow entries
- **THEN** the system returns a deny decision

### Requirement: Deny requests outside configured policy scope
The system SHALL deny a plan whose source, table, operation, or referenced columns are not fully covered by an applicable allow entry.

#### Scenario: Deny an inaccessible filter column
- **WHEN** a plan selects permitted columns but filters on a column absent from the grant
- **THEN** the system returns a deny decision before SQL compilation

#### Scenario: Deny an inaccessible source or table
- **WHEN** a plan names a source or table not covered by a policy for the trusted principal
- **THEN** the system returns a deny decision before SQL compilation

### Requirement: Explain authorization without leaking unavailable resources
The system SHALL provide a dry-run decision result containing allow or deny and a stable reason code; deny results SHALL NOT disclose policy names, hidden datasource names, hidden table names, or hidden column names.

#### Scenario: Explain an allowed plan
- **WHEN** a trusted caller requests an explanation for an allowed validated plan
- **THEN** the system returns allow, a stable allow reason code, and the matching policy identifier

#### Scenario: Explain a denied plan safely
- **WHEN** a caller requests an explanation for a denied validated plan
- **THEN** the system returns deny and a stable non-disclosing reason code without SQL execution

### Requirement: Evaluate one immutable configuration snapshot
The system SHALL evaluate each authorization attempt against one validated configuration snapshot and SHALL deny the attempt if no usable snapshot is available.

#### Scenario: Fail closed without usable policy configuration
- **WHEN** policy configuration is unavailable or invalid at evaluation time
- **THEN** the system returns a deny decision and does not create an authorized query
