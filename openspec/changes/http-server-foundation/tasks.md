# Tasks

## 1. HTTP configuration

- [ ] 1.1 Add optional `server.http.listenAddress` with the `127.0.0.1:8080` default; verify strict parser tests cover omitted, explicit, malformed, and unknown HTTP fields.
- [ ] 1.2 Update the example configuration with an explicit loopback listener and document the safe default inline; verify `datawark config validate` accepts both the example and an existing configuration without `server.http`.

## 2. HTTP boundary

- [ ] 2.1 Add `chi/v5` and construct one internal HTTP router/server with bounded header, idle, maximum-header, and shutdown limits; verify handler tests run through `httptest` without external sockets.
- [ ] 2.2 Implement `X-Request-ID` validation, generation, context propagation, and response headers; verify tests cover valid IDs, missing IDs, oversized values, unsafe characters, and non-reflection.
- [ ] 2.3 Add `/api/v1/health` and `/api/v1/ready` with explicit readiness state; verify tests cover ready serving and readiness withdrawal before shutdown.

## 3. Serve lifecycle

- [ ] 3.1 Add `datawark serve --config <path>` that completes runtime bootstrap before listener binding; verify a failed configuration or catalog bootstrap never invokes the listener factory and returns only a safe error.
- [ ] 3.2 Implement signal/context-driven graceful shutdown that stops new work and bounds in-flight completion; verify lifecycle tests use an ephemeral listener to cover startup, cancellation, readiness withdrawal, and forced deadline expiry.
- [ ] 3.3 Add an integration test that starts `serve`, calls both probes, and shuts it down cleanly; verify `go test ./...` and the focused integration test pass without exposing data routes.

## Workflow follow-up

- This change may be applied in parallel with `trusted-principal-runtime`.
- Apply and archive it before `http-api-key-authentication`, `gateway-audit-events`, `rest-data-api`, `mcp-streamable-http`, or `gateway-observability`.
