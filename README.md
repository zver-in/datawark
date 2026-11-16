# datawark
# DataWark Public Gateway MVP

DataWark is a self-hosted, read-only PostgreSQL gateway. It accepts a bounded Query Plan over REST and MCP Streamable HTTP, authenticates clients with API keys, applies YAML policy, and emits audit JSON to stdout.

## Prerequisites

- Docker with the Compose plugin
- `make` (for the release verification command)

## Quickstart

Create local-only secrets; `.env` is ignored by Git.

```sh
cp .env.example .env
# Replace every replace-with-a-long-random-* value in .env.
docker compose up --build --wait
curl -fsS http://127.0.0.1:8080/api/v1/ready
```

The demo policy permits the `demo-reader` API key to read four columns of `sales.orders`. With the value from `DATAWARK_LOCAL_API_KEY` in `.env`:

```sh
curl -sS http://127.0.0.1:8080/api/v1/query \
  -H "Authorization: Bearer $DATAWARK_LOCAL_API_KEY" \
  -H 'Content-Type: application/json' \
  --data '{"source":"analytics","table":"sales.orders","projection":["id","status","total","created_at"],"filters":[],"order":[],"limit":10}'
```

MCP clients connect to `http://127.0.0.1:8080/mcp` with the same Bearer API key and can call `list_allowed_schema`, `query_data`, and `explain_query`. Prometheus metrics are at `http://127.0.0.1:8080/metrics`.

Audit events are JSON lines on Gateway stdout; operational JSON logs are on stderr. Configure the container runtime or external collector to retain, rotate, and forward them. DataWark does not claim durable or exactly-once audit delivery.

Stop the demo and remove its data:

```sh
docker compose down --volumes
```

## Verification

```sh
make verify-public-mvp
```

The command creates runtime-only secrets, starts an isolated Compose project, runs black-box security tests, captures output, and removes containers, networks, volumes, and temporary secrets on exit.

## MVP exclusions

No Assistant, OIDC, write operations, hot reload, TLS termination, durable audit storage, extra database adapters, arbitrary SQL, or production orchestration manifest.
