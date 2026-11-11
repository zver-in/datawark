# Proposal

## Why

DataWark has product and architectural contracts for a secure PostgreSQL Gateway but no executable configuration boundary. M0 establishes a safe, testable CLI foundation so operators can detect invalid configuration and connectivity issues before the Policy Engine and query execution exist.

## What Changes

- Add a `datawark config validate --config <path>` command that strictly parses and validates the v1alpha1 Gateway configuration contract.
- Add a `datawark source test --config <path> --datasource <name>` command that obtains a PostgreSQL DSN only from the configured environment-variable name and verifies connectivity without reading user tables.
- Define field-path-bearing, non-secret diagnostic behavior and non-zero exits for invalid configuration or source-test failures.
- Add a safe example configuration and YAML fixtures covering the valid and required invalid cases.
- Extend unit, PostgreSQL integration, Make, and GitHub Actions coverage for the M0 commands.

## Capabilities

### New Capabilities

- `gateway-configuration`: Strict, secret-safe loading and semantic validation of the versioned Gateway YAML configuration.
- `datasource-connectivity`: CLI verification that a named, validated PostgreSQL datasource can be reached through its environment-provided DSN.

### Modified Capabilities

- None.

## Impact

Adds the Go CLI entry point and configuration/domain packages, a PostgreSQL connectivity adapter, configuration examples and test fixtures, and CI workflow coverage. It introduces Cobra, `go.yaml.in/yaml/v3`, and pgx v5 dependencies consistent with ARCH-001; it does not add Policy Engine, Query Plan, SQL compilation, REST/MCP, or schema discovery.
