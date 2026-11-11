# authorized-sql-compilation Specification

## Purpose

Определяет безопасную компиляцию уже авторизованного ограниченного запроса в один параметризованный PostgreSQL SELECT без доступа к неавторизованному клиентскому вводу.

## Requirements

### Requirement: Compile only an authorized Query Plan
The system SHALL compile SQL only from a Query Plan produced by a successful authorization decision and SHALL reject absent, denied, or malformed authorization output.

#### Scenario: Compile an allowed plan
- **WHEN** authorization produces an allowed Query Plan
- **THEN** the system produces one PostgreSQL SELECT statement and its bound argument list

#### Scenario: Refuse an untrusted plan
- **WHEN** compilation is requested without a successful authorization result
- **THEN** the system returns an error and produces no SQL statement

### Requirement: Produce a bounded read-only SELECT
The system SHALL compile at most one SELECT over the authorized `schema.table`, explicit projection, AND-combined filters, optional ordering, and limit; it SHALL not emit writes, DDL, joins, grouping, subqueries, functions, or multiple statements.

#### Scenario: Compile a projection with filters and ordering
- **WHEN** an authorized plan has projection, supported filters, order entries, and limit
- **THEN** the generated statement contains only the corresponding SELECT, FROM, WHERE, ORDER BY, and LIMIT clauses

#### Scenario: Compile a projection without optional clauses
- **WHEN** an authorized plan has no filters or ordering
- **THEN** the generated statement omits WHERE and ORDER BY while retaining an explicit LIMIT

### Requirement: Bind all plan values as PostgreSQL parameters
The system SHALL represent every filter value and limit as a positional PostgreSQL parameter, return those values separately in placeholder order, and SHALL NOT interpolate them into SQL text.

#### Scenario: Preserve a hostile filter value as an argument
- **WHEN** an authorized filter value contains quote characters or SQL-looking text
- **THEN** the generated SQL contains a placeholder and the exact value only in the bound arguments

#### Scenario: Parameterize the limit
- **WHEN** an authorized plan specifies a limit
- **THEN** the generated SQL represents the limit with a positional parameter rather than a numeric literal from the plan

### Requirement: Render identifiers from authorized components safely
The system SHALL render the authorized two-part table identifier and single-part column identifiers as PostgreSQL identifiers, and SHALL emit `ASC` or `DESC` only from an authorized ordering direction.

#### Scenario: Quote an unusual but authorized identifier
- **WHEN** an authorized identifier requires PostgreSQL quoting
- **THEN** the generated SQL quotes it as an identifier without treating it as executable SQL

#### Scenario: Reject an invalid identifier shape
- **WHEN** authorization output contains a table other than exactly two non-empty identifier parts or an empty column identifier
- **THEN** compilation fails without generating executable SQL
