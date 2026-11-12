# Design

## Context

The existing PostgreSQL adapter only connects and pings. M1 can validate policy and compile SQL, but its Query Plan has no guarantee that a granted `schema.table` or column exists. The configuration is loaded once at startup and supplies only datasource names and DSN environment-variable names.

## Goals / Non-Goals

**Goals:**

- Load a complete, immutable metadata snapshot for one configured datasource.
- Validate the metadata references of an already validated plan without running it.
- Derive principal-visible schema metadata from the snapshot and existing policy grants.

**Non-Goals:**

- Execute compiled queries or inspect application-row values.
- Create, migrate, or alter database objects.
- Add an HTTP, MCP, or CLI schema endpoint, schema caching, or hot reload.
- Replace policy authorization with schema visibility filtering.

## Decisions

### Introspect PostgreSQL catalog metadata through a read-only adapter

Add a PostgreSQL metadata adapter that resolves the selected datasource DSN from its configured environment variable, establishes a short-lived connection, and reads relation and column metadata from `pg_catalog`. It includes ordinary, partitioned, view, and materialized-view relations; it excludes catalog and `information_schema` namespaces from its returned model.

Using PostgreSQL catalog relations instead of probing each policy grant produces one coherent source snapshot and includes type names and nullability. The adapter never selects from an application relation and returns sanitized errors rather than driver/DSN details.

### Model immutable snapshots by datasource

Create schema domain types for datasource, qualified relation, and ordered columns. A snapshot is bound to exactly one configured datasource and is treated as immutable after load. Metadata validation rejects plans whose source differs from the snapshot before looking up relation or columns.

This matches the configuration's startup-only lifecycle and prevents accidental reuse of metadata across sources. Refresh/hot reload remains a later lifecycle concern.

### Keep schema validation downstream of grammar and policy

Schema validation receives an already validated plan; it verifies its referenced relation and column union against the snapshot without compiling or executing SQL. The eventual QueryService will order the path as validation, policy, metadata, compilation, execution.

Policy is still required: a snapshot records what the database role can see, not what the Gateway principal may see.

### Derive visible schema by intersecting grants with metadata

A visibility service takes a trusted principal, the same immutable configuration snapshot, and a schema snapshot. It enumerates select-only grants for that principal and returns only the intersection of granted and existing source/table/column metadata. It silently omits stale or unavailable policy objects.

This supports a future Schema API without exposing database-role visibility or using authorization errors as discovery hints.

## Risks / Trade-offs

- [Catalog metadata can change after startup] → bind the snapshot to request processing later and add an explicit refresh lifecycle only when required.
- [Database permissions may hide a relation] → treat it as absent and avoid inferring its existence from errors.
- [Policy names an object that does not exist] → omit it from discovery and reject queries during metadata validation; do not fail the entire unrelated snapshot.
- [Catalog query details are PostgreSQL-specific] → keep them inside the PostgreSQL adapter so future drivers define equivalent adapters.

## Migration Plan

The change introduces no persisted data and does not change configuration syntax. Deployment requires only that the existing read-only datasource role can read PostgreSQL metadata for the intended relations. Rollback removes the registry and leaves M1 behavior unchanged.
