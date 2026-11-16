# Public Gateway MVP threat model

## Assets and boundaries

Assets are API keys, datasource credentials, policy-visible rows, hidden relations/columns, audit events, operational logs, and metrics. Trust changes at client-to-Gateway HTTP, Gateway-to-PostgreSQL Compose network, and operator-to-local-secret boundaries. PostgreSQL is not host-published; Gateway binds to loopback by default.

## Mitigations

- API keys establish principal; supplied request identity is ignored.
- REST and MCP share policy evaluation and accept only a structured read plan.
- Unknown tables and columns are denied before protected rows return.
- Gateway uses a separate PostgreSQL reader role with no write ownership.
- Config references environment variables rather than embedding usable secrets.
- Audit/operational logs avoid credentials and protected values; metrics use bounded labels.

## Residual risks and exclusions

No TLS termination, OIDC, overlapping key rotation, rate limits, row-level policy, masking, durable audit sink, hot reload, production orchestration, arbitrary SQL, writes, Assistant, or extra databases. Operators must protect the loopback host, secret input, Docker socket, reverse proxy, backups, and log collector. Table/column policy does not prevent every inference from permitted data.
