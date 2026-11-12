# Design

## Context

M2 packages require callers to manually hold a validated config, a matching schema snapshot, and an executor. The CLI currently constructs only config and source-test commands, so a transport could otherwise assemble an incomplete or bypassable path.

## Goals / Non-Goals

**Goals:**

- Build all runtime dependencies once from one validated configuration.
- Make schema discovery, dry-run explanation, and query execution transport-neutral operations.
- Fail closed if any configured datasource lacks a usable snapshot.

**Non-Goals:**

- Hot reload, background schema refresh, network serving, persistent state, or authentication beyond the configured development principal.

## Decisions

### Bootstrap every datasource before serving

Runtime construction loads configuration once and creates a metadata snapshot per datasource. Construction succeeds only when every snapshot succeeds, so a transport never gets a partial map. This favours the PRD fail-closed invariant over partial availability.

### Own immutable configuration and snapshots in one runtime

The runtime retains private copies and resolves an operation's datasource to its snapshot. It builds `QueryService` internally, so adapters receive operations rather than database or compiler dependencies.

### Add transport-neutral schema and explain services

Schema listing intersects grants and metadata through `schema.Visible`. Explain validates the plan, evaluates policy and metadata, then returns a safe outcome without invoking the compiler or executor. This makes `explain_query` and future REST behavior identical.

### Use the configured development principal exclusively

The runtime derives the principal from the immutable configuration. Public method inputs never include a principal, role, DSN, or policy identifier.

## Risks / Trade-offs

- [One unavailable datasource prevents local MCP start] -> Deliberate fail-closed behavior; source-test remains available for diagnosis.
- [Snapshots become stale] -> Reload is explicitly out of scope; the next lifecycle change must define atomic refresh semantics.
- [Runtime grows into a transport-specific facade] -> Keep its operations in domain-shaped Go types and add no MCP imports.

## Migration Plan

Add the runtime package without changing YAML. A future CLI/MCP command uses it at startup; rollback removes those commands and leaves M2 packages usable by existing tests.
