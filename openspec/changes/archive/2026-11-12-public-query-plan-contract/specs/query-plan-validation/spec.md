# Spec Delta

## ADDED Requirements

### Requirement: Define the canonical public Query Plan JSON form
The system SHALL accept public Query Plans only with the top-level fields `source`, `table`, `projection`, `filters`, `order`, and `limit`; their meanings and values SHALL match the Query Plan grammar.

#### Scenario: Decode the canonical field names
- **WHEN** a caller supplies a complete plan using `projection` and `order`
- **THEN** the system decodes and validates that plan according to the bounded Query Plan grammar

#### Scenario: Reject a documented legacy alias
- **WHEN** a caller supplies `select`, `columns`, `orderBy`, or `order_by` instead of a canonical field
- **THEN** the system rejects the plan as unsupported input before authorization

### Requirement: Preserve one JSON representation across public transports
The system SHALL use the canonical field names and the existing JSON scalar, filter, ordering, and limit encodings in every public Query Plan input schema.

#### Scenario: Publish a transport input schema
- **WHEN** a public transport exposes a Query Plan input
- **THEN** its schema names and constrains fields according to the canonical public Query Plan JSON form
