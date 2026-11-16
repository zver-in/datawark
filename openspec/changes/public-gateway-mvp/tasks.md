# Tasks

## 1. Package the loopback-only demonstration deployment

- [ ] 1.1 Add a multi-stage `Dockerfile` that builds the Go binary and runs Gateway as a non-root user with a readiness probe utility; verify `docker build` succeeds and the container user is not root.
- [ ] 1.2 Add root `compose.yaml`, `.env.example`, and `deploy/gateway.yaml` so Gateway is published only on a configurable loopback host port, PostgreSQL has no host port, and all credentials/API keys are environment references; verify `docker compose config` contains no usable secret and rejects missing required values safely.
- [ ] 1.3 Add PostgreSQL initialization assets that create a seeded demonstration relation, separate owner and Gateway reader roles, and only the reader grants needed by the example policy; verify the documented query succeeds while a reader-role write is denied.
- [ ] 1.4 Configure PostgreSQL and Gateway healthchecks plus dependency ordering; verify `docker compose up --build --wait` reaches Gateway readiness and `docker compose down --volumes` removes the stack.

## 2. Add packaged security and release verification

- [ ] 2.1 Add `make verify-public-mvp` to generate runtime-only test secrets, build an isolated Compose project, wait with bounded deadlines, run release tests, capture output, and clean up containers, networks, and volumes on every exit path; verify the command completes without external services.
- [ ] 2.2 Add black-box REST security regressions under `tests/security/` for readiness, absent/malformed/multiple/unknown credentials, client-selected identity, hidden relation/column access, and rejected requests; verify each case has the stable denial response and returns no protected row.
- [ ] 2.3 Add black-box MCP HTTP and observability regressions against the built Compose stack; verify an authenticated MCP call succeeds, REST/MCP decisions agree, and distinctive credentials or protected values are absent from responses, audit stdout, operational stderr, and `/metrics`.
- [ ] 2.4 Add database-boundary regressions that execute a reader-role write through the Compose service and check the seeded data after negative Gateway scenarios; verify writes are denied and the expected rows remain unchanged.

## 3. Publish the Public Gateway operating contract

- [ ] 3.1 Replace the placeholder README with prerequisites, secret setup, Compose quickstart, readiness check, allowed REST request, MCP example, metrics, shutdown, and explicit MVP exclusions; verify the commands work unchanged through `make verify-public-mvp`.
- [ ] 3.2 Add operations guidance covering configuration, API-key rotation procedure, DB least privilege, loopback exposure, health/readiness, and audit stdout versus operational stderr collection; verify it makes no durability or exactly-once claim for audit events.
- [ ] 3.3 Add `docs/security/threat-model.md` and `SECURITY.md` describing assets, trust boundaries, mitigations, residual risks, supported version/reporting policy, and out-of-scope protections; verify they match the existing API-key, policy, database, audit, and observability specifications.
- [ ] 3.4 Update PRD and architecture documentation to mark M4 complete, describe M5 as the Public Gateway release, and remove stale statements that HTTP/API-key functionality is planned; verify documented scope still excludes Assistant, OIDC, writes, hot reload, and extra databases.

## 4. Enforce the release gate in CI

- [ ] 4.1 Consolidate the duplicate GitHub Actions workflows into one least-privilege workflow that runs formatting/vet, race unit tests, PostgreSQL integration tests, and `make verify-public-mvp` for pull requests and `main`; verify a workflow lint or GitHub Actions parse accepts the resulting file and only one CI workflow remains.
- [ ] 4.2 Run `make check`, `make test`, `make test-integration`, `make verify-public-mvp`, `openspec validate --all --strict`, and `git diff --check`; verify every command passes and no Compose resources remain after verification.

## Workflow follow-up

- Review the Public Gateway MVP artifacts and implementation evidence before archiving this change.
- Archive the change only after the M5 release gate passes in CI.
