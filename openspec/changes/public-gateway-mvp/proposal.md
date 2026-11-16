# Proposal

## Why

M4 provides an authenticated and tested HTTP Gateway, but it is not yet a reproducible self-hosted product: the repository has no release container or application Compose stack, its README is only a placeholder, and no release gate verifies the packaged deployment. M5 turns the completed Gateway into a documented Public MVP with an explicit operational and security boundary.

## What Changes

- Add a non-root container image and a two-service Docker Compose quickstart that runs Gateway and PostgreSQL with no host-published database port, a dedicated read-only Gateway database role, loopback-only host exposure, and secrets supplied only through untracked environment input.
- Supply an example Gateway configuration and database seed that let a new operator make one allowed authenticated request while preserving the existing API-key, policy, audit, and read-only execution contracts.
- Publish a complete README and operations material covering installation, configuration, API-key use, REST/MCP examples, metrics, audit collection, safe shutdown, and the supported MVP boundary.
- Add a threat model and a security policy that state trusted boundaries, residual risks, reporting expectations, and explicitly unsupported capabilities.
- Add black-box security regression and release verification for the built image and Compose stack; make it a single, reproducible CI gate alongside the existing Go checks and remove duplicate CI execution.
- Synchronize product and architecture status documentation to record M4 as complete and M5 as the Public Gateway MVP release.

## Capabilities

### New Capabilities

- `self-hosted-deployment`: Reproducible, secret-safe local Docker Compose deployment of the Public Gateway with a read-only PostgreSQL boundary.
- `public-gateway-release-gates`: Black-box packaged-deployment security regressions and the required CI release gate.

### Modified Capabilities

None.

## Impact

Adds root-level container and Compose assets, demo configuration and PostgreSQL initialization material, environment examples, release/security tests, Make targets, one consolidated GitHub Actions workflow, and README, operations, security, product, and architecture documentation. It does not change public Gateway routes, Query Plan semantics, authentication rules, or add Assistant, OIDC, writes, hot reload, extra databases, or advanced data-governance features.
