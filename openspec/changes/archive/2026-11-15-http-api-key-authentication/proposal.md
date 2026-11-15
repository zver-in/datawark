# Proposal

## Why

The HTTP Gateway must not publish data operations under the local development principal. M4 needs a simple self-hosted authentication boundary that maps server-verified API keys to configured policy principals without introducing a metadata database.

## What Changes

- Add static API-key-to-principal mappings to HTTP configuration, storing only environment-variable names in YAML.
- Resolve all configured key secrets during bootstrap and fail closed on missing, empty, duplicate, or ambiguous credentials.
- Authenticate protected HTTP requests from the Bearer authorization scheme using constant-time secret comparison.
- Attach only the server-derived trusted principal to downstream runtime operations.
- Keep liveness, readiness, and the bounded metrics endpoint unauthenticated while rejecting every data-plane or unknown request without a valid key.
- Exclude dynamic key issuance, persistence, administration, OIDC, and client-selected principals.

## Capabilities

### New Capabilities

- `api-key-authentication`: Static secret-safe HTTP API key authentication and trusted-principal derivation.

### Modified Capabilities

- `gateway-configuration`: Validate API key mappings under the HTTP configuration without accepting secret values in YAML.

## Impact

This depends on `trusted-principal-runtime` and `http-server-foundation`. It affects configuration parsing, bootstrap, HTTP middleware, error handling, and security tests, and becomes a prerequisite for REST and MCP Streamable HTTP.
