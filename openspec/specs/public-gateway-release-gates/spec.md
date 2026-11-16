# public-gateway-release-gates Specification

## Purpose

Defines reproducible release verification for the containerized Public Gateway MVP and its security boundaries.

## Requirements

### Requirement: Verify the packaged deployment end to end
The repository SHALL provide one documented release-verification command that builds the Gateway image, starts the Public Gateway Compose stack, waits for readiness, verifies an authenticated REST request and authenticated MCP HTTP operation, and cleans up test resources.

#### Scenario: Run public MVP verification
- **WHEN** a maintainer runs the documented release-verification command on a Docker-capable checkout
- **THEN** it verifies the built image and Compose deployment without using an external database or service

### Requirement: Regress critical unauthenticated and authorization denials
The packaged-deployment security suite SHALL verify that missing, malformed, multiple, and unknown API-key credentials; client-supplied identity; and requests for non-visible resources are rejected before protected data is returned.

#### Scenario: Submit negative security cases
- **WHEN** the release suite sends each documented negative authentication or authorization request to the running Compose stack
- **THEN** each request receives its stable rejection outcome and no protected row is returned

### Requirement: Regress the read-only database boundary
The packaged-deployment security suite SHALL verify that the Gateway datasource role cannot write to the demonstration data and that rejected Gateway requests do not change that data.

#### Scenario: Attempt to modify demo data
- **WHEN** the release suite exercises direct datasource-role writes and rejected Gateway requests
- **THEN** PostgreSQL denies the write and the demonstration data retains its expected state

### Requirement: Verify secret-safe packaged observability
The release suite SHALL verify that distinctive test credentials and protected values are absent from Gateway error responses, audit output, operational logs, and Prometheus metrics produced by the Compose deployment.

#### Scenario: Exercise hostile input in the packaged stack
- **WHEN** the release suite sends a rejected request containing distinctive credentials and protected-resource values
- **THEN** none of those values appears in the captured response, audit stream, operational stream, or metrics exposition

### Requirement: Enforce one public MVP release gate in CI
The repository CI SHALL run the public MVP verification together with the existing formatting, static analysis, unit, and integration checks for pull requests and changes to the main branch, without duplicate workflow execution.

#### Scenario: Validate a proposed release change
- **WHEN** a pull request or main-branch change is evaluated by CI
- **THEN** the change cannot pass the configured release gate unless all required Public Gateway verification commands succeed once
