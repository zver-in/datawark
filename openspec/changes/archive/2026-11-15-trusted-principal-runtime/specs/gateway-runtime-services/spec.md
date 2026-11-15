# Spec Delta

## MODIFIED Requirements

### Requirement: Execute a public plan only through the security pipeline
The runtime SHALL execute a public Query Plan only through validation, authorization for a trusted principal supplied by the server-side caller, metadata validation, authorized compilation, and read-only execution; it SHALL NOT derive identity from Query Plan content.

#### Scenario: Execute an allowed runtime query
- **WHEN** a caller submits an allowed valid plan together with a server-trusted principal for a loaded datasource
- **THEN** the runtime returns the labelled read-only result from the completed security pipeline for that principal

#### Scenario: Reject a denied runtime query
- **WHEN** validation, authorization for the supplied trusted principal, metadata validation, compilation, or execution cannot proceed
- **THEN** the runtime returns a safe failure without bypassing an earlier gate

#### Scenario: Ignore untrusted identity content
- **WHEN** request content attempts to supply or override a principal or role
- **THEN** the runtime does not use that content as identity and authorizes only with the server-trusted principal

### Requirement: Provide policy-filtered schema and dry-run operations
The runtime SHALL list only datasource and relation metadata visible to the server-trusted principal supplied for the operation and SHALL explain a plan for that same principal without executing SQL.

#### Scenario: List visible metadata
- **WHEN** a caller requests schema metadata with a server-trusted principal for a loaded datasource
- **THEN** the runtime returns only the relations and columns visible to that principal

#### Scenario: Explain without execution
- **WHEN** a caller requests an explanation with a server-trusted principal and a plan
- **THEN** the runtime returns the safe allow or deny outcome after applicable validation and metadata checks without opening an execution transaction
