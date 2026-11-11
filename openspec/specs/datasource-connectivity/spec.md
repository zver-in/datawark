# datasource-connectivity Specification

## Purpose

Provides a safe operator-facing check that a configured PostgreSQL datasource is reachable without accessing application data.

## Requirements

### Requirement: Test a named configured datasource
The system SHALL provide `datawark source test --config <path> --datasource <name>`, validate the supplied configuration before use, and select only the datasource whose name matches the requested name.

#### Scenario: Test a reachable PostgreSQL datasource
- **WHEN** an operator names a declared PostgreSQL datasource and its configured environment variable contains a working PostgreSQL DSN
- **THEN** the command verifies connectivity and exits with code zero

#### Scenario: Reject an unknown datasource name
- **WHEN** an operator requests a datasource name not declared in the configuration
- **THEN** the command exits non-zero with a diagnostic identifying the datasource argument

#### Scenario: Reject invalid configuration before connecting
- **WHEN** an operator runs source test with a configuration that violates the configuration contract
- **THEN** the command exits non-zero without attempting a database connection

### Requirement: Resolve DSNs exclusively from environment variables
Source testing SHALL read the DSN only from the environment variable named by the selected datasource's `dsnEnv`; it SHALL reject an unset or empty variable and SHALL NOT accept a DSN from YAML or command-line arguments.

#### Scenario: Report a missing DSN environment value safely
- **WHEN** the selected datasource's `dsnEnv` variable is unset or empty
- **THEN** the command exits non-zero without printing the DSN or a password

#### Scenario: Keep connection failures secret-safe
- **WHEN** PostgreSQL rejects or cannot establish the connection described by the environment-provided DSN
- **THEN** the command exits non-zero without emitting the DSN or embedded credentials

### Requirement: Limit source testing to connectivity
The source-test operation SHALL perform only PostgreSQL connection health verification and SHALL NOT issue queries against user tables, views, or other application data.

#### Scenario: Confirm a source without user-table access
- **WHEN** source test runs against the integration PostgreSQL instance
- **THEN** it succeeds using connectivity verification without querying a user table
