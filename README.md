# DataWark

DataWark is a self-hosted, read-only gateway for PostgreSQL. It lets applications and AI agents query a deliberately small, policy-controlled part of a database without receiving database credentials or arbitrary SQL access.

Clients submit a structured Query Plan over REST or MCP. DataWark authenticates the client, validates the request, checks a YAML allowlist, compiles an authorized plan into parameterized SQL, and executes it with a least-privilege PostgreSQL role.

## What it provides

- PostgreSQL read access constrained by YAML policies for principals, data sources, tables, and columns.
- A bounded Query Plan: one table, explicit projections, simple `AND` filters, ordering, and a required limit. Arbitrary SQL is not accepted.
- REST API with Bearer API-key authentication.
- MCP Streamable HTTP and MCP stdio, with `list_allowed_schema`, `query_data`, and `explain_query` tools.
- Filtered schema discovery, so callers see only resources allowed by their policy.
- Runtime limits for query time, returned rows, result size, and concurrent queries.
- JSON audit events on stdout, operational JSON logs on stderr, and Prometheus metrics at `/metrics`.

## Quick start

The demo uses Docker Compose and binds the gateway to `127.0.0.1`, so it is available only from the local machine by default.

### Prerequisites

- Docker with the Compose plugin
- `curl`

### Start the demo

Create local secrets from the supplied template. Do not reuse the example values outside a local demo.

```sh
cp .env.example .env
# Edit .env and replace every replace-with-a-long-random-* value.
docker compose up --build --wait
```

Check that the gateway is ready:

```sh
curl --fail http://127.0.0.1:8080/api/v1/ready
```

To use the API key stored in `.env` in the current shell:

```sh
set -a
. ./.env
set +a
```

If you changed `DATAWARK_GATEWAY_PORT`, replace `8080` in the examples below.

## Use the REST API

The demo API key is allowed to read only `id`, `status`, `total`, and `created_at` from `sales.orders`.

List the schema visible to the authenticated principal:

```sh
curl --fail --silent \
  -H "Authorization: Bearer $DATAWARK_LOCAL_API_KEY" \
  http://127.0.0.1:8080/api/v1/schema
```

Run a query:

```sh
curl --fail --silent \
  -X POST http://127.0.0.1:8080/api/v1/query \
  -H "Authorization: Bearer $DATAWARK_LOCAL_API_KEY" \
  -H 'Content-Type: application/json' \
  --data '{
    "source": "analytics",
    "table": "sales.orders",
    "projection": ["id", "status", "total", "created_at"],
    "filters": [{"column": "status", "operator": "eq", "value": "paid"}],
    "order": [{"column": "created_at", "direction": "desc"}],
    "limit": 10
  }'
```

Check whether a plan is allowed without running it:

```sh
curl --fail --silent \
  -X POST http://127.0.0.1:8080/api/v1/query/explain \
  -H "Authorization: Bearer $DATAWARK_LOCAL_API_KEY" \
  -H 'Content-Type: application/json' \
  --data '{
    "source": "analytics",
    "table": "sales.orders",
    "projection": ["id"],
    "filters": [],
    "order": [],
    "limit": 1
  }'
```

The complete REST contract is in [api/openapi.yaml](api/openapi.yaml).

## Use MCP

Point an MCP client that supports Streamable HTTP at:

```text
http://127.0.0.1:8080/mcp
```

Configure it to send `Authorization: Bearer <DATAWARK_LOCAL_API_KEY>`. The server provides these tools:

- `list_allowed_schema`
- `query_data`
- `explain_query`

For a trusted local operator, the same tools are also available over stdio:

```sh
go run ./cmd/datawark mcp stdio --config deploy/gateway.yaml
```

MCP stdio uses the configured development principal. HTTP callers must authenticate with an API key.

## Configure your own database

The Compose demo configuration is [deploy/gateway.yaml](deploy/gateway.yaml). It declares:

- `server.http.apiKeys`: API-key principal mappings; each key is read from an environment variable.
- `datasources`: PostgreSQL connections, with DSNs supplied through environment variables rather than tracked YAML.
- `policies`: explicit table, operation, and column allowlists.
- `server.runtime`: query timeout and response/concurrency limits.

With Go 1.27 or later installed, validate a custom configuration, test the database connection, and start the gateway:

```sh
go run ./cmd/datawark config validate --config path/to/gateway.yaml
go run ./cmd/datawark source test --config path/to/gateway.yaml --datasource analytics
go run ./cmd/datawark serve --config path/to/gateway.yaml
```

For deployment and API-key rotation guidance, see [Public Gateway operations](docs/operations/public-gateway.md). For the security model and operational boundary, see the [threat model](docs/security/threat-model.md) and [security policy](SECURITY.md).

## Verification

```sh
make check
make test
make test-integration
make verify-public-mvp
```

`make verify-public-mvp` creates isolated runtime secrets, launches the demo stack, runs black-box security checks, and removes its containers, network, volumes, and temporary secrets when it exits.

## MVP boundaries

DataWark does not support writes, arbitrary SQL, joins, aggregates, OIDC, TLS termination, hot reload, durable audit storage, additional database engines, or a built-in AI assistant. Place a TLS-aware, authenticated reverse proxy in front of the local Compose deployment before exposing it beyond the host.

Stop the demo and remove its data with:

```sh
docker compose down --volumes
```
