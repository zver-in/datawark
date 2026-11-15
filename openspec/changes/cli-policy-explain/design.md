# Design

## Context

The current Cobra tree has configuration validation, datasource testing, and MCP stdio. Runtime explain already performs validation, authorization, and metadata checks without execution. After the trusted-principal change, local adapters obtain a facade bound to the configured development principal.

## Goals / Non-Goals

**Goals:**

- Expose the existing safe explanation path as a scriptable local command.
- Use exactly the public canonical Query Plan decoder.
- Separate completed deny decisions from command failures.

**Non-Goals:**

- Execute queries, choose arbitrary principals, administer policies, or start HTTP.
- Provide a human-oriented interactive formatter in M4.

## Decisions

### Use one required `--plan` source

Add `policy explain --config <path> --plan <path|->`. A path reads a bounded file; `-` reads bounded stdin. The command accepts exactly one JSON document up to 1 MiB and uses `query.Decode`, so aliases and unknown fields behave exactly like public transports.

Flags were chosen over embedding JSON on the command line to avoid shell history exposure and quoting ambiguity.

### Bind to the local runtime principal

Bootstrap the complete runtime, obtain its local facade bound to `server.developmentPrincipal`, and call explain. No `--principal`, role environment variable, or plan identity field exists. This keeps the command consistent with the initial local trust model.

### Treat deny as a successful explanation

Write one versioned JSON object containing `allowed`, stable `reason`, and `policyId` only for allowed results. A completed allow or deny exits zero. Malformed input, configuration/bootstrap failure, or output failure exits non-zero with a concise safe stderr message.

### Prove the execution boundary with injected dependencies

Construct the command from narrow loader/runtime/input interfaces, as existing CLI tests do. Unit and integration tests attach an executor that fails the test if called, proving the explain path never compiles or executes SQL.

## Risks / Trade-offs

- [Loading all datasource snapshots makes explain depend on database availability] → Preserve the current immutable runtime contract; an offline policy-only mode would have different metadata semantics and is out of scope.
- [Machine-readable output is less friendly interactively] → Keep a stable JSON contract first; a human formatter can be added later without changing evaluation.

## Migration Plan

1. Apply `trusted-principal-runtime`.
2. Add policy command construction and bounded plan-source reading.
3. Map runtime explanation to the versioned JSON DTO and safe exits.
4. Add file, stdin, deny, malformed input, secret-redaction, and no-execution tests.

Rollback removes the command without affecting runtime or other transports.
