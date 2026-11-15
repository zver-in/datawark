# Proposal

## Why

Custom applications and the future DataWark Assistant need a versioned HTTP API over the completed Gateway runtime. The REST adapter must preserve the same canonical Query Plan and enforcement path as MCP without gaining direct infrastructure access.

## What Changes

- Add authenticated `POST /api/v1/query` and `POST /api/v1/query/explain` endpoints.
- Add authenticated `GET /api/v1/schema` discovery and `GET /api/v1/schema/{source}` detail endpoints.
- Decode only the canonical Query Plan JSON form and delegate exclusively to principal-aware runtime services.
- Return stable non-sensitive JSON envelopes with request IDs and bounded request bodies.
- Publish the complete versioned REST contract as an OpenAPI document kept in sync by validation tests.
- Integrate terminal audit events and prove REST/MCP parity for equivalent principals and plans.
- Exclude policy administration, datasource administration, raw SQL, browser sessions, and API-key management.

## Capabilities

### New Capabilities

- `rest-data-api`: Versioned authenticated REST access to policy-filtered schema, query execution, and dry-run explanation.

### Modified Capabilities

- None.

## Impact

This adds REST routing and transport DTOs on the shared HTTP listener. It depends on `trusted-principal-runtime`, `http-server-foundation`, `http-api-key-authentication`, and `gateway-audit-events`.
