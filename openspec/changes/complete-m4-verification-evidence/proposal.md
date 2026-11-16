# Proposal

## Why

M4 behavior is implemented and the accepted specifications remain valid, but three acceptance claims are not yet demonstrated at the boundary where they matter: real `serve` process streams, every prohibited metric-cardinality dimension, and Docker-backed REST/MCP metric and sink isolation. Add the missing evidence so verification status is based on executable tests rather than completed task markers.

## What Changes

- Add a child-process `serve` test that captures the real process stdout and stderr, proving stdout contains only complete audit JSON lines while stderr contains parseable operational JSON for startup, request completion, readiness withdrawal, and shutdown in order.
- Exercise many distinct principals, request IDs, API-key values, sources, tables, columns, paths and path parameters, status/error text, and other arbitrary user-controlled values through supported HTTP boundaries, then parse Prometheus output and prove the owned metric series remain bounded and contain none of those inputs.
- Strengthen the Docker-backed PostgreSQL scenario to assert the exact bounded REST and MCP HTTP metric series created by authenticated traffic and to capture audit and operational sinks independently.
- Keep accepted requirements, production behavior, public endpoints, metric names and labels, CLI behavior, and delivery guarantees unchanged.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

None. This change adds verification evidence for the accepted `gateway-observability`, `gateway-audit-events`, `http-server-lifecycle`, `rest-data-api`, and `mcp-streamable-http` contracts without changing their requirements.

## Impact

The change is limited to process-level CLI tests, HTTP observability tests, Docker-backed integration tests, and only the narrow test seams needed to capture existing audit and operational outputs independently. It introduces no new runtime dependency, endpoint, configuration field, persisted data, or compatibility change.
