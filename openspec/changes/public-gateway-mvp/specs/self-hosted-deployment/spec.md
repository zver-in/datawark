# Spec Delta

## Purpose

Defines a reproducible and secret-safe Docker Compose deployment for a local DataWark Public Gateway MVP.

## ADDED Requirements

### Requirement: Run an isolated Public Gateway Compose stack
The repository SHALL provide a documented Compose stack that starts Gateway and PostgreSQL together, exposes only the Gateway HTTP port on the host loopback interface, and leaves PostgreSQL reachable only on the Compose network.

#### Scenario: Start the documented stack
- **WHEN** an operator supplies the required environment values and starts the documented Compose command
- **THEN** Gateway becomes ready and accepts HTTP traffic through the documented loopback endpoint

#### Scenario: Attempt host access to PostgreSQL
- **WHEN** a process outside the Compose network attempts to connect to PostgreSQL through a host port
- **THEN** no PostgreSQL host port is available

### Requirement: Keep deployment secrets outside tracked configuration
The Compose assets and Gateway configuration SHALL contain only environment-variable references for database credentials and API keys, and SHALL fail safely when required secret values are unavailable.

#### Scenario: Inspect tracked deployment assets
- **WHEN** an operator inspects the committed Compose, configuration, and environment-example files
- **THEN** none contains a usable database credential, API key, or DSN value

#### Scenario: Omit a required deployment secret
- **WHEN** an operator starts the stack without a required database credential or API key
- **THEN** startup fails without publishing a ready Gateway or reflecting the missing value

### Requirement: Demonstrate a least-privilege database boundary
The supplied demonstration database SHALL initialize an application owner separately from the Gateway datasource role, and the datasource role SHALL have only the SELECT privileges required by the documented policy.

#### Scenario: Execute a permitted demo query
- **WHEN** an authenticated caller submits the documented permitted query
- **THEN** Gateway returns only the policy-visible columns from the demonstration relation

#### Scenario: Attempt a write as the Gateway role
- **WHEN** the datasource role attempts to modify the demonstration relation
- **THEN** PostgreSQL rejects the operation and the relation remains unchanged

### Requirement: Provide an executable quickstart
The repository SHALL document a fresh-clone quickstart that creates local secret input, starts the Compose stack, waits for readiness, and performs one authenticated allowed Gateway request without requiring external services.

#### Scenario: Follow the quickstart on a clean checkout
- **WHEN** an operator follows the documented quickstart with Docker Compose available
- **THEN** the operator reaches a ready Gateway and receives the documented allowed response

### Requirement: Document operational output ownership
The self-hosted documentation SHALL distinguish Gateway audit JSON emitted on stdout from operational JSON emitted on stderr and SHALL state that persistence, rotation, and forwarding after stdout handoff belong to the runtime or external collector.

#### Scenario: Configure audit collection
- **WHEN** an operator configures log collection for the Compose deployment
- **THEN** the documentation identifies the audit stream and does not claim durable or exactly-once Gateway delivery
