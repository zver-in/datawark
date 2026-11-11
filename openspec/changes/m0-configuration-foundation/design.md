# Design

## Context

The repository currently contains only module metadata, Make targets, and a TCP-level PostgreSQL integration scaffold. PRD-001 and ARCH-001 define the v1alpha1 YAML shape, a Cobra CLI, YAML v3, pgx v5, and a Docker Compose integration boundary. See proposal.md and the two delta specs for the behavioral contract.

## Goals / Non-Goals

**Goals:**

- Establish one typed configuration load-and-validate path used by both CLI commands.
- Make invalid input actionable through field-path diagnostics while preventing secret disclosure.
- Test a configured PostgreSQL endpoint without accessing application relations.
- Preserve the existing `make test`, `make test-integration`, and `make ci` separation.

**Non-Goals:**

- Policy evaluation, query planning, SQL compilation, schema discovery, hot reload, and network service transports.
- Credential storage, DSN values in configuration, DSN command flags, or automatic secret provisioning.
- Verifying database grants, tables, schema metadata, or executing an application query.

## Decisions

### Typed configuration boundary with two validation phases

Use typed Go domain/configuration structures for the supported v1alpha1 shape: `server.developmentPrincipal`, `datasources`, `policies`, and policy `allow` entries. Decode YAML with known-field enforcement and reject malformed YAML, duplicate mapping keys, unknown fields, unsupported scalar forms, and trailing documents before semantic validation.

Semantic validation then checks required strings, unique datasource and policy names, supported version/driver/operation values, and policy datasource references. Validation errors are collected as structured field-path violations (for example, `datasources[1].dsnEnv`) and rendered by the CLI without serializing the complete configuration or process environment.

This separates syntax/schema errors from cross-reference rules and lets future loaders reuse the same safe model. A permissive map-based parser was rejected because it makes unknown fields and stable error paths unreliable; a generic JSON Schema dependency was rejected because the small versioned contract is clearer and safer as typed Go validation.

### Cobra command tree with a shared configuration service

Introduce `cmd/datawark` as the binary entry point and root command, with `config validate` and `source test` subcommands. Both require `--config`; source test additionally requires `--datasource`. Command construction receives narrow interfaces for loading configuration, environment lookup, and connectivity, so unit tests can assert exits and output without a real database.

The CLI emits concise operator diagnostics to stderr and returns a non-zero error for validation, source selection, environment, or connectivity failure. Success output names only the action/datasource. Raw decoder, pgx, and environment errors must be wrapped or classified before presentation, since they may contain a connection string. Passing original errors through unchanged was rejected due to credential-leak risk.

### Environment-only DSN resolution and PostgreSQL health check

After successful validation and datasource lookup, source test resolves exactly `os.LookupEnv(datasource.dsnEnv)` and rejects a missing or empty value. The DSN remains an in-memory value passed directly to a PostgreSQL connectivity adapter; it is never included in configuration structs' serialized output, logs, diagnostics, test assertion messages, or CLI flags.

Use pgx to establish and ping a PostgreSQL connection, close it promptly, and issue no SQL against user tables or views. A `SELECT 1` health query is not required for this M0 boundary; driver connection establishment plus protocol-level ping is sufficient and avoids expanding the observable database-access surface. Opening a pool or running catalog/table probes was rejected because source test needs no data access.

### Test artifacts and CI alignment

Add a valid redacted example under `configs/examples/`, using only environment-variable names. Add YAML fixtures for the valid case and each listed validation failure: unknown field, empty `dsnEnv`, invalid driver, invalid operation, duplicate datasource/policy names, and unknown policy datasource. Unit tests exercise decoding, semantic rules, paths, exit behavior, and secret redaction without Docker.

Adapt the existing integration Compose setup and `make test-integration` so an integration test invokes the source-test path with `DATAWARK_TEST_POSTGRES_DSN` and proves success against PostgreSQL. Add a GitHub Actions workflow that runs `make ci`; CI credentials remain the existing disposable test credentials and do not enter example configuration.

## Risks / Trade-offs

- [Decoder or driver errors contain a DSN] → normalize external errors before presentation and add regression assertions using credential-bearing test DSNs.
- [Field paths from YAML syntax errors are less precise than semantic paths] → retain decoder location when available and consistently identify the nearest safe configuration path.
- [A TCP-only check would not prove PostgreSQL protocol readiness] → use pgx connection/ping rather than the current raw TCP integration check.
- [Fixed host port can conflict on developer machines] → preserve the established Compose contract for M0 and ensure teardown runs after integration tests.
- [Future configuration versions need different rules] → dispatch validation by explicit `version`, rejecting all versions other than v1alpha1 until a new version is designed.

## Migration Plan

1. Add the CLI, typed configuration contract, fixtures, example, and tests without changing an existing runtime configuration format.
2. Add pgx-backed source testing and update the integration test to invoke the built command path.
3. Add the GitHub Actions workflow to run `make ci` on pushes and pull requests.
4. Roll back by removing the new binary/commands and dependencies; no persisted data or configuration migration is introduced.
