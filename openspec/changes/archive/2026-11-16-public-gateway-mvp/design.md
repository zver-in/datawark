# Design

## Context

See `proposal.md` for motivation. M4 already provides `datawark serve`, API-key authentication, REST, MCP Streamable HTTP, read-only PostgreSQL execution, audit JSON on stdout, operational JSON on stderr, and health/readiness endpoints. The repository has only a test PostgreSQL Compose file; it has no root Dockerfile or application Compose stack, README is `TBD`, `tests/security/` is empty, and two CI workflow files overlap.

The existing configuration deliberately names environment variables rather than containing DSNs or API keys. Gateway must complete PostgreSQL catalog bootstrap before readiness; an application Compose stack therefore needs a seeded relation and a datasource credential with enough metadata and SELECT access, but no write ownership.

## Goals / Non-Goals

**Goals:**

- Deliver a local, reproducible Public Gateway Compose experience that is safe by default on the developer host.
- Test the image and its runtime topology from outside the process, including security and observability boundaries.
- Make the operating model, residual risks, and release criteria explicit and actionable.

**Non-Goals:**

- Add a new Gateway route, configuration field, identity provider, policy language feature, database adapter, hot reload, or Assistant deployment.
- Provide a production orchestrator manifest, TLS termination, secret manager integration, durable audit storage, a database migration framework, or multi-instance operation.
- Claim that the demo database setup substitutes for an operator's own least-privilege review.

## Decisions

### Ship a two-service, loopback-only Compose topology

Add a root `Dockerfile`, `compose.yaml`, `.env.example`, `deploy/gateway.yaml`, and PostgreSQL initialization material under `deploy/`. The Gateway image will use a multi-stage Go build and an Alpine runtime that has an HTTP probe utility; it will run as a non-root user. Its Compose configuration binds Gateway to `0.0.0.0:8080` inside the container, while the host publishes it only as `127.0.0.1:<configured-port>`. PostgreSQL receives no `ports` entry and is addressed only by its Compose service name.

The stack will use `depends_on` with PostgreSQL health and a Gateway readiness healthcheck, making `docker compose up --wait` meaningful. A distroless runtime was rejected because the current binary has no local health-probe subcommand; adding one solely to support image healthchecking would expand the public CLI instead of using a minimal runtime tool.

### Separate demonstration ownership from the Gateway role

PostgreSQL initialization will create an application-owner role for schema/data setup and a separate login role used exclusively by the Gateway datasource DSN. The reader receives USAGE on the demo schema and SELECT only on the relation and allowed columns; it does not own the schema, table, or database. Initialization uses values injected from the operator's local environment rather than committed credentials.

Using the PostgreSQL bootstrap superuser as the datasource role was rejected because it would invalidate the product's database-defense-in-depth claim. Publishing the database port for convenience was rejected because the Gateway is the intended access boundary.

### Preserve the existing secret and audit boundaries

Tracked configuration will contain only variable names such as `DATAWARK_ANALYTICS_DSN` and `DATAWARK_LOCAL_API_KEY`. `.env.example` will explain required values but carry placeholders, and the quickstart will create a local ignored `.env` with operator-chosen secrets. Missing values must make Compose or Gateway startup fail before a ready endpoint is exposed.

The documentation will identify stdout as the audit stream and stderr as operational logging. It will state that the container runtime or collector is responsible for retaining, rotating, and forwarding audit data after the documented stdout handoff. Treating `docker compose logs` as a durable audit sink was rejected because it cannot provide persistence or exactly-once guarantees.

### Make release verification black-box and self-cleaning

Add `make verify-public-mvp` to build the image, start an isolated Compose project with runtime-generated distinctive test secrets, wait for readiness, run `tests/security/`, capture outputs, and clean up containers, volumes, and networks on success or failure. The suite will use the published HTTP port for REST/MCP/metrics checks and `docker compose exec` for the database-role write check, so PostgreSQL remains unpublished.

The suite will cover the release-specific successful path, negative authentication/authorization cases, unchanged demo data, no secret leakage in response/audit/log/metrics surfaces, and REST/MCP behavior. Reusing only package-level M4 tests was rejected because they do not verify the final image, environment wiring, database role, or Compose network exposure.

### Consolidate CI and document the product boundary

Keep one GitHub Actions workflow and have it run `make check`, `make test`, `make test-integration`, and `make verify-public-mvp` for pull requests and `main`. Update the README with a fresh-clone quickstart and REST/MCP examples; add an operations guide, `docs/security/threat-model.md`, and `SECURITY.md`; and align PRD/ARCH milestone and runtime-status language with M4 completion and M5 scope.

A separate release system, image registry publication, SBOM/signing, and vulnerability scanning are deferred: they may be added later without changing the Public Gateway behavior defined here.

## Risks / Trade-offs

- [Demo credentials could be mistaken for production defaults] → Use placeholders only in tracked files and state prominently that operators must supply unique local values.
- [Host port 8080 is already occupied] → Make the loopback host port configurable through the operator environment while preserving the loopback bind.
- [Compose startup races database initialization] → Use PostgreSQL and Gateway healthchecks; regression tests wait for readiness with bounded timeouts.
- [Docker logs mix streams in a viewer] → Document stream ownership and test captured stdout/stderr separately; require an external collector for operational persistence.
- [Security tests become flaky or leave resources behind] → Use unique Compose project names, explicit deadlines, and unconditional cleanup traps.
- [A release gate substantially increases CI duration] → Keep the demo dataset small, reuse cached image layers where available, and retain focused M4 unit/integration suites for faster diagnosis.

## Migration Plan

1. Add packaging, deployment configuration, and least-privilege demo database assets without changing existing runtime contracts.
2. Add the release-verification target and security suite; run it locally and in the consolidated CI workflow.
3. Publish the quickstart, operations guidance, threat model, security policy, and synchronized milestone documentation.
4. Announce the Public Gateway MVP only after the complete CI gate passes from a clean checkout.

Rollback removes the new distribution assets and release gate; no production data migration or API migration is required. Operators who used the demo stack remove it with `docker compose down --volumes`.
