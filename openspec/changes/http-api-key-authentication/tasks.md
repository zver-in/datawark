# Tasks

## 1. API key configuration

- [x] 1.1 Extend `server.http` with strict `apiKeys` entries containing `principal` and `keyEnv`; verify parser tests reject empty values, duplicate principals, duplicate environment names, inline secrets, and unknown fields.
- [x] 1.2 Update the example configuration with a redacted API-key environment reference; verify validation succeeds without reading the secret and no fixture contains a key value.

## 2. Credential bootstrap

- [x] 2.1 Implement bootstrap resolution that hashes configured key values and retains only fixed-size digests mapped to trusted principals; verify tests cover distinct secrets and secret values are absent from returned structures and diagnostics.
- [x] 2.2 Fail bootstrap for missing, empty, or duplicate secret values before listener binding; verify credential-bearing regression tests prove neither keys nor digests appear in errors or output.

## 3. HTTP authentication

- [x] 3.1 Implement strict single-Bearer parsing and constant-time digest comparison across configured keys; verify tests cover scheme casing, malformed headers, multiple headers, unknown keys, and the mapped principal.
- [x] 3.2 Attach the authenticated trusted principal to request context and reject identity override attempts from other request content; verify downstream fake handlers observe only the key-mapped identity.
- [x] 3.3 Wrap the HTTP router with a uniform unauthorized JSON response and `WWW-Authenticate`, exempting only health, readiness, and `/metrics`; verify anonymous tests cover each operational endpoint, data-shaped routes, and unknown routes.

## 4. Security integration

- [x] 4.1 Integrate credential bootstrap into `datawark serve`; verify the server binds only after both runtime and authentication dependencies are complete.
- [x] 4.2 Add HTTP integration tests for two keys mapped to different policy principals and for hostile credential values; verify identity isolation and secret redaction under `go test ./...` and `make test-integration`.

## Workflow follow-up

- Apply and archive `trusted-principal-runtime` and `http-server-foundation` first.
- Archive this change before `gateway-audit-events`, `rest-data-api`, or `mcp-streamable-http`.
