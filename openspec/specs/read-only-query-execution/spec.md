# read-only-query-execution Specification

## Purpose

Выполняет полностью проверенный Query Plan в PostgreSQL и возвращает результаты чтения, не создавая пути для пользовательского SQL, записи или обхода policy и schema checks.

## Requirements

### Requirement: Execute only a fully verified Query Plan
The system SHALL execute a query only after validating its Query Plan, deriving the trusted server principal, allowing it through policy evaluation, validating it against the datasource schema snapshot, and compiling it through the authorized SQL compiler.

#### Scenario: Execute a permitted existing query
- **WHEN** a Query Plan passes validation, policy, schema validation, and compilation for the configured development principal
- **THEN** the system executes the resulting statement against its named datasource and returns its rows

#### Scenario: Reject before execution
- **WHEN** validation, policy, metadata validation, or compilation fails
- **THEN** the system returns an error without opening an execution transaction for the plan

### Requirement: Execute statements in a read-only transaction
The system SHALL execute the compiled statement in an explicit PostgreSQL read-only transaction and SHALL execute exactly one compiler-produced statement per query request.

#### Scenario: Complete a read-only query
- **WHEN** a compiler-produced SELECT succeeds
- **THEN** the system commits or closes its read-only transaction and returns the result

#### Scenario: Prevent a write through the execution path
- **WHEN** an attempted statement would write data or alter database state
- **THEN** the system does not execute it through the query service

### Requirement: Return labelled result rows
The system SHALL return result columns in statement order and rows whose values correspond to those columns, preserving database null values distinctly from non-null values.

#### Scenario: Return rows with null values
- **WHEN** an allowed query returns one or more rows containing a database NULL
- **THEN** the response identifies its columns and represents the NULL value without converting it to a string

#### Scenario: Return an empty result
- **WHEN** an allowed query matches no rows
- **THEN** the response contains the selected columns and an empty row set

### Requirement: Keep execution diagnostics secret-safe
The system SHALL return stable execution failure categories without exposing a DSN, credentials, compiled SQL text, bound parameter values, or ungranted database object names.

#### Scenario: Report a database failure safely
- **WHEN** PostgreSQL returns an execution or connection failure
- **THEN** the system returns a safe execution error without exposing connection or statement details
