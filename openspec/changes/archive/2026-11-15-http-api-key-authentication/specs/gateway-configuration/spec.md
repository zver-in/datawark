# Spec Delta

## ADDED Requirements

### Requirement: Validate static HTTP API key mappings
The system SHALL accept one or more `server.http.apiKeys` entries containing a non-empty principal and `keyEnv`, SHALL store no key value in YAML, and SHALL reject duplicate principals, duplicate environment-variable names, empty fields, and unknown mapping fields.

#### Scenario: Accept a secret reference
- **WHEN** an API key mapping contains a unique configured principal and the name of an environment variable
- **THEN** strict configuration validation accepts the mapping without resolving or serializing the secret value

#### Scenario: Reject an ambiguous mapping
- **WHEN** API key mappings repeat a principal or `keyEnv` name or contain an empty required field
- **THEN** validation fails with the applicable field path

#### Scenario: Reject an inline key value
- **WHEN** configuration supplies an unsupported inline API key field
- **THEN** strict validation rejects it without reflecting the value
