# Proposal

## Why

The M4 observability and CLI explanation changes were archived before every stated acceptance condition was demonstrated by the implementation and tests. Close those verification gaps now so archived task completion matches the already accepted behavior contracts.

## What Changes

- Align HTTP completion telemetry with the accepted safe outcome and supported-operation contract, including schema-detail requests.
- Strengthen lifecycle, Prometheus parsing, execution-category, cardinality, and telemetry-isolation tests so they exercise every acceptance dimension named by the archived observability tasks.
- Add Docker-backed REST and MCP HTTP traffic coverage that scrapes `/metrics` and verifies audit/operational stream separation.
- Complete CLI explanation failure and no-compilation/no-execution verification for valid and invalid plans without changing its public JSON or command contract.
- Keep all existing accepted requirements, public endpoints, CLI flags, metric names, and security boundaries unchanged.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

None. The accepted `gateway-observability` and `cli-policy-explain` requirements remain authoritative; this change corrects implementation and verification only.

## Impact

The change affects HTTP telemetry classification and completion logging, observability and CLI test seams, unit tests, and Docker-backed integration coverage. It introduces no new public capability, configuration field, endpoint, or compatibility break.
