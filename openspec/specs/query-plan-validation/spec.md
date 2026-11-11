# query-plan-validation Specification

## Purpose

Определяет ограниченный и однозначно проверяемый структурированный запрос, который Gateway может безопасно передать в авторизацию без поддержки произвольного SQL.

## Requirements

### Requirement: Accept a bounded single-table Query Plan
The system SHALL accept a Query Plan only when it names one non-empty source and `schema.table`, has a non-empty unique projection, and contains only filters, ordering, and a positive integer limit defined by this capability.

#### Scenario: Validate a bounded read plan
- **WHEN** a plan contains one source, one qualified table, selected columns, supported filters, ordering, and a positive limit
- **THEN** the system returns a validated Query Plan without executing or compiling SQL

#### Scenario: Reject an incomplete plan
- **WHEN** a plan omits its source, table, projection, or has an empty projected column
- **THEN** the system rejects it with a field-path-bearing validation error

### Requirement: Restrict expressions to typed AND filters
The system SHALL permit filters only as an AND list of column comparisons using `eq`, `neq`, `lt`, `lte`, `gt`, or `gte` and scalar string, number, boolean, or null values; null SHALL be allowed only with `eq` or `neq`.

#### Scenario: Validate supported filter comparisons
- **WHEN** a plan uses supported comparison operators with permitted scalar values
- **THEN** the system preserves the ordered filter list in the validated plan

#### Scenario: Reject an unsupported expression
- **WHEN** a plan contains a nested expression, function, aggregate, subquery, array value, object value, unsupported operator, or null with an ordered comparison
- **THEN** the system rejects the plan before policy evaluation

### Requirement: Restrict ordering and limit syntax
The system SHALL permit zero or more order entries only for non-empty column names and `asc` or `desc` directions, and SHALL reject a missing, zero, negative, fractional, or non-numeric limit.

#### Scenario: Validate deterministic ordering
- **WHEN** a plan orders by named columns using `asc` or `desc`
- **THEN** the system returns the ordering entries in their submitted order

#### Scenario: Reject invalid order or limit
- **WHEN** a plan uses an unsupported direction or an invalid limit value
- **THEN** the system rejects the plan with the offending field path

### Requirement: Fail closed for unsupported Query Plan content
The system SHALL reject unknown fields and every construct outside the supported Query Plan grammar, including raw SQL, joins, grouping, aggregation, and multiple tables, without reflecting submitted values in diagnostics.

#### Scenario: Reject an attempted raw SQL field
- **WHEN** a request supplies a raw SQL field or any unknown Query Plan field
- **THEN** the system rejects the request before authorization and does not include the submitted value in the error

#### Scenario: Defer database metadata checks
- **WHEN** a syntactically valid plan refers to a table or column whose database existence is not yet known
- **THEN** Query Plan validation does not connect to PostgreSQL and leaves metadata verification to a later schema capability
