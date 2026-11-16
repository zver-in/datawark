# Public Gateway operations

## Configuration and exposure

Copy `.env.example` to an untracked `.env` and replace every placeholder with unique values. `DATAWARK_GATEWAY_PORT` changes the host port, but Compose always binds it to `127.0.0.1`; PostgreSQL has no host-published port. To expose Gateway beyond the host, place an authenticated TLS-aware reverse proxy in front of it and review the threat model first.

`deploy/gateway.yaml` references only `DATAWARK_ANALYTICS_DSN` and `DATAWARK_LOCAL_API_KEY`. The DSN is assembled inside Compose from reader-role credentials. Do not put DSNs or API keys in tracked YAML.

## API-key rotation

1. Generate a high-entropy replacement and update `DATAWARK_LOCAL_API_KEY` in local secret storage or `.env`.
2. Distribute it securely to authorized clients.
3. Restart Gateway: `docker compose up -d --force-recreate gateway`.
4. Confirm readiness and an authenticated request, then revoke the old client key.

The MVP supports one demo key; rotation requires a planned restart and has no overlap window.

## Database, health, and logs

The init script creates a non-login owner and a separate Gateway login role. Gateway has schema USAGE and SELECT only on four visible columns; it owns neither database, schema, nor table. Review grants, RLS, network boundaries, and migrations for production separately.

`/api/v1/health` reports process health; `/api/v1/ready` succeeds after bootstrap. Compose waits for PostgreSQL health and probes Gateway readiness.

Audit JSON is stdout; operational JSON is stderr. Collect streams separately where possible. Retention, rotation, forwarding, durability, ordering, and exactly-once delivery belong to the runtime or external collector, not DataWark.
