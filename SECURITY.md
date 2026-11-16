# Security policy

## Supported version

Only the current `main` development version is supported during the Public Gateway MVP. Update before reporting an issue.

## Reporting a vulnerability

Do not open a public issue for a suspected vulnerability. Contact project maintainers privately with the affected revision, reproduction, impact, and mitigation ideas. Allow time for triage and coordinated remediation before disclosure.

## Security boundary

The supported boundary includes API-key authentication, YAML policy, read-only PostgreSQL access, REST/MCP parity, audit JSON, and bounded metrics. It excludes TLS, OIDC, writes, durable audit delivery, hot reload, row-level controls, Assistant, and additional database adapters. See [the threat model](docs/security/threat-model.md).
