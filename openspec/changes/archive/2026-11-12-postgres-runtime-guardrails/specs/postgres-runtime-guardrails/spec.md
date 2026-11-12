# Spec Delta

## Purpose

Ограничивает время, параллелизм и объём результатов PostgreSQL read execution, чтобы разрешённый запрос не истощал ресурсы Gateway или datasource.

## ADDED Requirements

### Requirement: Enforce a statement deadline
The system SHALL apply the configured statement timeout to every PostgreSQL query execution and SHALL return a stable timeout failure when the deadline is exceeded.

#### Scenario: Cancel an overlong statement
- **WHEN** an execution exceeds its configured statement timeout
- **THEN** the system cancels or rolls back that execution and returns a timeout failure without partial results

### Requirement: Bound returned rows and bytes
The system SHALL reject an execution when its requested row limit exceeds the configured maximum row count or when collecting its result would exceed the configured maximum result bytes; it SHALL not return partial results.

#### Scenario: Reject an excessive requested row limit
- **WHEN** a validated Query Plan has a limit greater than the configured maximum rows
- **THEN** the system rejects the request before SQL execution

#### Scenario: Reject an oversized result
- **WHEN** an execution's collected rows exceed the configured result-byte limit
- **THEN** the system stops the execution, discards collected rows, and returns a result-size failure

### Requirement: Bound concurrent executions
The system SHALL allow no more than the configured maximum number of concurrent PostgreSQL executions and SHALL reject excess requests without waiting indefinitely or opening a transaction.

#### Scenario: Reject a request at capacity
- **WHEN** the configured execution concurrency capacity is occupied
- **THEN** the system returns a stable capacity failure and does not send the new request to PostgreSQL

### Requirement: Clean up resources after every bounded execution
The system SHALL release concurrency capacity and close or roll back execution resources after success, timeout, cancellation, size rejection, and database failure.

#### Scenario: Reuse capacity after a failure
- **WHEN** an execution ends in timeout or another execution failure
- **THEN** a later permitted request can acquire the released capacity
