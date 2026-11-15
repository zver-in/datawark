# Tasks

## 1. REST contract and decoding

- [ ] 1.1 Define explicit REST DTOs and stable status/error-code mappings for query, explain, schema, authentication, validation, denial, guardrail, unavailable, and internal outcomes; verify table-driven tests cover every safe category without raw errors.
- [ ] 1.2 Implement 1 MiB single-document `application/json` decoding through the canonical Query Plan decoder; verify tests reject unknown fields, legacy aliases, trailing JSON, unsupported media types, oversized bodies, and reflected input.
- [ ] 1.3 Create `api/openapi.yaml` for Bearer auth, request IDs, four routes, canonical plans, success DTOs, and error envelopes; verify an OpenAPI parser accepts the document and contract tests enumerate the same method/path pairs as the router.

## 2. Query and explanation routes

- [ ] 2.1 Implement authenticated `POST /api/v1/query` over the principal-bound runtime; verify allowed results preserve columns, rows, and nulls while denied or failed requests return stable correlated errors.
- [ ] 2.2 Implement authenticated `POST /api/v1/query/explain` without compilation or execution; verify allow and deny return completed explanation DTOs and an injected executor is never called.
- [ ] 2.3 Wrap query and explain in terminal operation auditing before successful body commit; verify writer failure discards results and returns unavailable exactly once.

## 3. Schema routes

- [ ] 3.1 Implement authenticated `GET /api/v1/schema` over principal-filtered datasource listing and snapshots; verify only visible relations and columns appear for each test principal.
- [ ] 3.2 Implement authenticated `GET /api/v1/schema/{source}` with a uniform hidden/unknown/unavailable response; verify path values and hidden metadata are never reflected.
- [ ] 3.3 Add schema operation audit integration and OpenAPI response checks; verify collection and detail routes emit one safe terminal event each.

## 4. Transport integration

- [ ] 4.1 Mount all REST routes behind request ID and API-key middleware on the shared listener; verify probes remain anonymous and all REST data routes require authentication.
- [ ] 4.2 Add Docker-backed end-to-end tests for allowed query, deny, explain, schema filtering, runtime guardrails, audit failure, and two-principal isolation; verify `make test-integration` passes.
- [ ] 4.3 Add parity tests comparing REST with MCP stdio for equivalent principals and plans; verify allow, deny, and stable failure categories match.

## Workflow follow-up

- Apply and archive `trusted-principal-runtime`, `http-server-foundation`, `http-api-key-authentication`, and `gateway-audit-events` first.
- This change may be applied in parallel with `mcp-streamable-http`.
