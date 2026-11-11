# Design

## Context

The prior M1 changes establish validated plans and policy decisions but intentionally do not execute SQL. ARCH-001 requires a compiler that sees only an authorized internal query and emits parameterized PostgreSQL SQL.

## Goals / Non-Goals

**Goals:**

- Create a narrow capability boundary from successful authorization to one read-only statement.
- Preserve typed values as bound arguments and produce deterministic placeholder ordering.
- Quote identifiers safely even before M2 adds schema metadata.

**Non-Goals:**

- Open a database connection, execute statements, or enforce transaction settings.
- Add joins, expressions, aggregation, arbitrary functions, or raw SQL.
- Treat compiler quoting as a replacement for policy or schema enforcement.

## Decisions

### Require an opaque authorized query

Introduce an internal authorized-query representation whose construction is owned by the successful authorization path. The compiler accepts this representation rather than the public or merely validated plan.

Keeping the representation opaque to transports prevents a future adapter from calling the compiler with unchecked client data. Go package visibility cannot protect against every internal misuse, so tests and package dependencies also enforce the intended one-way flow.

### Compile from fixed clause templates

Build only `SELECT <columns> FROM <schema>.<table>`, optional `WHERE` joined by `AND`, optional `ORDER BY`, and `LIMIT`. Operators and directions are selected from closed enums. No user-controlled fragment becomes a SQL clause.

This is safer and easier to fuzz than a general SQL AST or string passthrough.

### Quote identifiers and parameterize every literal

Split the table into exactly two identifier parts and quote each PostgreSQL identifier, including embedded quote escaping. Emit values and limit as `$1..$N` arguments in filter order followed by limit. Never interpolate a scalar value.

Metadata validation in M2 will ensure identifiers exist; quoting here independently prevents them from changing SQL grammar.

### Make compilation deterministic and side-effect free

Expose a compiled statement value containing SQL text and arguments. Repeated compilation of the same authorized plan yields identical SQL and argument order, enabling unit and fuzz tests without PostgreSQL.

## Risks / Trade-offs

- [Quoted unknown identifiers fail only at execution] → M2 schema validation runs before the executor and produces a controlled error.
- [An internal opaque type is not a process-level security boundary] → retain policy enforcement and DB least privilege; do not expose compiler to transports.
- [PostgreSQL-specific placeholders] → keep this compiler behind a PostgreSQL adapter boundary; other dialects need separate compilers.

## Migration Plan

This change adds no database migrations or runtime endpoint. Rollback removes the compiler capability and leaves the preceding validation and policy artifacts intact.
