# Spec Delta

## MODIFIED Requirements

### Requirement: Validate a versioned Gateway configuration
The system SHALL provide `datawark config validate --config <path>` and accept only a YAML document whose version is `v1alpha1` and whose document, mapping, and sequence fields conform to the supported configuration schema, including optional `server.runtime` limits.

#### Scenario: Validate a complete supported configuration
- **WHEN** an operator validates a YAML file with `version: v1alpha1`, server, datasource, and policy values conforming to the supported schema
- **THEN** the command exits with code zero

#### Scenario: Reject unknown configuration content
- **WHEN** an operator validates YAML containing an unknown field at any supported configuration level
- **THEN** the command exits non-zero and identifies the field path

#### Scenario: Reject unsupported document version
- **WHEN** an operator validates a YAML file whose version is absent or differs from `v1alpha1`
- **THEN** the command exits non-zero and identifies the version field path

#### Scenario: Validate configured runtime limits
- **WHEN** an operator provides positive `server.runtime.statementTimeout`, `maxRows`, `maxResultBytes`, and `maxConcurrentQueries` values
- **THEN** validation accepts the configuration and retains each limit

#### Scenario: Reject an invalid runtime limit
- **WHEN** a runtime limit is non-positive, malformed, or an unknown field
- **THEN** validation exits non-zero and identifies the runtime field path

### Requirement: Enforce datasource constraints
Each datasource SHALL have a non-empty unique name, `driver: postgres`, and a non-empty `dsnEnv` that names the environment variable holding its connection string.

#### Scenario: Reject an unsupported datasource driver
- **WHEN** a datasource specifies a driver other than `postgres`
- **THEN** validation exits non-zero and identifies that datasource driver's field path

#### Scenario: Reject a missing DSN environment variable name
- **WHEN** a datasource has an empty or absent `dsnEnv`
- **THEN** validation exits non-zero and identifies its `dsnEnv` field path

#### Scenario: Reject duplicate datasource names
- **WHEN** two datasources use the same name
- **THEN** validation exits non-zero and identifies the duplicate name's field path

### Requirement: Enforce policy references and select-only grants
Each policy SHALL have a non-empty unique name, non-empty principals, and a datasource that names a declared datasource. Every allow entry SHALL have a non-empty table and columns list and SHALL permit only the `select` operation.

#### Scenario: Reject an unknown policy datasource
- **WHEN** a policy refers to a datasource name absent from the datasource list
- **THEN** validation exits non-zero and identifies the policy datasource field path

#### Scenario: Reject unsupported policy operation
- **WHEN** an allow entry includes an operation other than `select`
- **THEN** validation exits non-zero and identifies the operation field path

#### Scenario: Reject duplicate policy names
- **WHEN** two policies use the same name
- **THEN** validation exits non-zero and identifies the duplicate name's field path

### Requirement: Keep secrets out of configuration diagnostics
The configuration format SHALL store only the `dsnEnv` identifier, not a DSN value. Validation failures and command output SHALL NOT emit a DSN, database password, or the value of any environment variable.

#### Scenario: Report an invalid configuration without leaking a secret
- **WHEN** validation fails while the process environment contains a PostgreSQL DSN with credentials
- **THEN** the non-zero command output contains no DSN or password value

## ADDED Requirements

### Requirement: Apply compatible runtime limit defaults
The system SHALL use `5s` statement timeout, 1,000 maximum rows, 4,194,304 maximum result bytes, and 16 maximum concurrent queries when corresponding `server.runtime` fields are absent.

#### Scenario: Use defaults for an existing configuration
- **WHEN** an operator validates an existing supported configuration without `server.runtime`
- **THEN** validation succeeds and the runtime configuration contains the documented defaults
