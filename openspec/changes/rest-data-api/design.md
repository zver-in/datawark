# Design

## Context

The principal-bound runtime already supplies transport-neutral query, schema, and explain operations. HTTP lifecycle, authentication, request correlation, and audit are separate M4 changes. The REST adapter must remain a thin translation layer and publish a versioned contract.

## Goals / Non-Goals

**Goals:**

- Map four v1 REST routes to a principal-bound Gateway facade.
- Keep decoding, errors, and schema visibility strict and non-disclosing.
- Make the OpenAPI document an acceptance artifact rather than informal documentation.

**Non-Goals:**

- Policy or datasource administration, pagination, streaming rows, CORS policy, browser sessions, or raw SQL.
- Reimplementing validation, authorization, schema filtering, or execution.

## Decisions

### Mount a small handler set on the shared chi router

Create a REST adapter that depends on a function returning the request's principal-bound Gateway facade plus the audit recorder. It imports no PostgreSQL, compiler, policy configuration, or environment access. The API-key middleware runs before route dispatch.

### Reuse canonical decoding with a bounded body

Require `application/json` with optional charset, limit request bodies to 1 MiB, read exactly one document, and call the existing canonical Query Plan decoder. Close and discard bodies safely. Query and explain use the same request schema.

### Use collection and detail schema routes

`GET /api/v1/schema` enumerates visible datasource snapshots. `GET /api/v1/schema/{source}` returns one filtered snapshot. Hidden, unknown, and unavailable names share one not-found envelope. Path decoding is bounded and never becomes identity.

### Define stable success and error DTOs

Success responses use explicit DTOs for query result, explanation, and schema metadata. Errors use `{error:{code,message,requestId}}`. Stable mappings distinguish invalid input, unauthenticated, denied, not found, timeout, capacity, result limits, unavailable, and internal failure without embedding underlying error text. Explain denial is a successful explanation response, not a transport failure.

### Audit before committing successful bodies

Handlers compute a result into memory, finalize the terminal audit event, and only then write a successful status and body. Audit failure discards the result and writes unavailable. Error outcomes are also finalized once through the handler wrapper.

### Maintain `api/openapi.yaml`

Publish OpenAPI 3.x definitions for Bearer security, request IDs, all four paths, canonical Query Plan, success DTOs, and error envelopes. Contract tests parse the document and compare registered route/method pairs plus representative payloads to handler behavior.

## Risks / Trade-offs

- [Buffered results increase memory use] → PostgreSQL result bytes are already bounded; reuse that bound before audit and JSON encoding.
- [Manual DTOs can drift from runtime values] → Keep conversion functions narrow and validate representative responses against OpenAPI.
- [A 1 MiB request limit may be larger than current plans need] → It is a hard upper bound, while Query Plan validation remains much stricter.

## Migration Plan

1. Apply all four prerequisite Changes.
2. Add DTOs, safe error classification, body decoding, and route handlers.
3. Mount routes behind authentication and operation-audit wrappers.
4. Add OpenAPI and contract tests.
5. Add end-to-end parity tests against MCP stdio for equivalent identities.

Rollback unmounts `/api/v1/query`, `/api/v1/query/explain`, and schema routes while retaining the HTTP server and probes.
