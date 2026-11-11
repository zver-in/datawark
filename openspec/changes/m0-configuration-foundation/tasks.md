# Tasks

## 1. Configuration contract and validation

- [x] 1.1 Add the v1alpha1 typed configuration model for server, datasources, policies, and allow entries; verify `go test ./internal/config/...` compiles and passes.
- [x] 1.2 Implement strict YAML decoding that rejects malformed input, duplicate keys, unknown fields, unsupported structures, and trailing documents; verify unit tests cover each rejected input class.
- [x] 1.3 Implement semantic validation for required fields, supported version/driver/operation values, unique datasource and policy names, and policy datasource references; verify unit tests assert every violation's field path.
- [x] 1.4 Define sanitized diagnostic errors so validation never renders DSNs, passwords, or environment values; verify credential-bearing unit inputs do not appear in errors.
- [x] 1.5 Add valid and invalid YAML fixtures (unknown field, empty `dsnEnv`, invalid driver/operation, duplicate names, and unknown policy datasource); verify the configuration test suite consumes each fixture.

## 2. CLI configuration validation

- [x] 2.1 Add the `datawark` Cobra root and `config validate --config <path>` command backed by the shared loader/validator; verify a valid fixture exits zero and each invalid fixture exits non-zero.
- [x] 2.2 Render concise CLI errors with safe field paths and exit statuses while keeping raw decoder/configuration content out of output; verify command-level tests assert the secret-redaction contract.
- [x] 2.3 Add a redacted v1alpha1 configuration example under `configs/examples/` that uses only `dsnEnv`; verify it validates when its named environment variable is absent (validation does not require the DSN value).

## 3. PostgreSQL source connectivity

- [x] 3.1 Add a PostgreSQL connectivity adapter that establishes and pings a pgx connection, closes it, and exposes no table/query operation; verify adapter tests use a fake boundary where practical.
- [x] 3.2 Add `source test --config <path> --datasource <name>` to load validated configuration, resolve only the selected `dsnEnv` with `os.LookupEnv`, and invoke the connectivity adapter; verify unit tests cover unknown source, missing/empty env, and invalid config before connection.
- [x] 3.3 Sanitize source-test environment and PostgreSQL failures before CLI rendering; verify tests with a credential-bearing DSN confirm neither DSN nor password is output.
- [x] 3.4 Update the integration test to run source test against the Compose PostgreSQL service through `DATAWARK_TEST_POSTGRES_DSN` and prove success without user-table access; verify `make test-integration` passes.

## 4. Delivery checks

- [x] 4.1 Add GitHub Actions coverage that runs `make ci` for pull requests and pushes; verify the workflow uses the repository Make target.
- [x] 4.2 Run formatting, static checks, unit tests, and the Docker-backed integration test through `make ci`; verify the full command passes locally.

## Workflow follow-up

- Archive the change after implementation and review are complete.
