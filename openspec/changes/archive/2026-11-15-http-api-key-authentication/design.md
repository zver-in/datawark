# Design

## Context

After the foundation change, the HTTP listener has only operational endpoints and a complete runtime. The trusted-principal change provides a bound facade but does not establish identity. Configuration must remain secret-free and no metadata database is available for credential lifecycle.

## Goals / Non-Goals

**Goals:**

- Resolve static credentials before network exposure and map each to one policy principal.
- Authenticate all non-operational HTTP traffic uniformly.
- Keep credentials and reusable derivatives out of every output channel.

**Non-Goals:**

- Dynamic issuance, rotation without restart, expiration, usage quotas, OIDC, or browser authentication.
- API-key administration endpoints or persisted credential state.

## Decisions

### Configure principals and secret references, never secret values

Extend `server.http` with `apiKeys`, each containing `principal` and `keyEnv`. Strict validation rejects empty and duplicate names. The HTTP bootstrap resolves every environment variable before binding, hashes each secret with SHA-256 in memory, and rejects duplicate digests.

Hashing creates fixed-length comparison inputs and avoids retaining raw key strings. Digests remain credentials and are never logged, serialized, or audited.

### Parse one Bearer credential and compare all candidates constantly

Authentication accepts exactly one `Authorization` field with a case-insensitive `Bearer` scheme and one non-empty credential. It hashes the submitted credential and compares fixed-size digests using constant-time comparison without early success return across candidates. Missing, malformed, multiple, and unknown credentials share one response shape.

### Store identity in request context as a trusted value

Successful middleware attaches the identity value produced by the authentication package. REST and MCP handlers must obtain a principal-bound runtime facade from that value. Request bodies, paths, ordinary headers, and MCP arguments never participate in identity resolution.

### Exempt an explicit operational allowlist

Only `/api/v1/health`, `/api/v1/ready`, and `/metrics` bypass authentication. The metrics route is added later but reserved now. Authentication wraps the router so unknown paths are protected rather than revealing the route surface anonymously.

### Keep failures uniformly secret-safe

Return a generic JSON unauthorized envelope with the effective request ID and `WWW-Authenticate: Bearer`; do not distinguish missing and incorrect keys. Bootstrap failures name only the safe configuration field or environment-variable identifier, never its value.

## Risks / Trade-offs

- [Static keys require restart for rotation] → Treat restart-based rotation as the explicit MVP lifecycle and defer dynamic management.
- [Configuration principal names reveal authorization structure locally] → Configuration is an operator-controlled artifact; secrets remain in the environment.
- [Linear comparison cost grows with key count] → Keep the MVP key set operationally small; fixed-time scans avoid identity-dependent timing and are sufficient for self-hosted M4.

## Migration Plan

1. Apply `http-server-foundation` and `trusted-principal-runtime` first.
2. Extend configuration and example fixtures with secret references.
3. Add bootstrap resolver and authentication middleware before mounting data routes.
4. Verify probes remain anonymous and every other route fails closed.

Rollback removes HTTP key mappings and middleware; no credential database or migration exists.
