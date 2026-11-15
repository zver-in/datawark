# Spec Delta

## MODIFIED Requirements

### Requirement: Execute only a fully verified Query Plan
The system SHALL execute a query only after validating its Query Plan, receiving a trusted principal from the server-side identity boundary, allowing that principal through policy evaluation, validating the plan against the datasource schema snapshot, and compiling it through the authorized SQL compiler.

#### Scenario: Execute a permitted existing query
- **WHEN** a Query Plan passes validation, policy evaluation for the supplied trusted principal, schema validation, and compilation
- **THEN** the system executes the resulting statement against its named datasource and returns its rows

#### Scenario: Reject before execution
- **WHEN** trusted identity is unavailable or validation, policy, metadata validation, or compilation fails
- **THEN** the system returns an error without opening an execution transaction for the plan
