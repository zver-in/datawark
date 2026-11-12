# Design

## Context

Gateway Runtime will expose transport-neutral schema, explain, and query operations over the M2 security pipeline. The repository has Cobra CLI but no MCP dependency, adapter, or bootstrap command.

## Goals / Non-Goals

**Goals:**

- Provide local stdio MCP interoperability without a listener.
- Keep each tool a thin decoding and response-mapping adapter over Runtime.
- Make protocol and security regressions testable without a real AI client.

**Non-Goals:**

- MCP Streamable HTTP, REST, remote authentication, tool-based configuration changes, streaming rows, or partial results.

## Decisions

### Use the Go MCP SDK with stdio transport

Add the architecture-selected `modelcontextprotocol/go-sdk` and register one server with three tools. The SDK owns JSON-RPC framing; the command supplies process stdin/stdout. Hand-written JSON-RPC would duplicate protocol compatibility work.

### Keep MCP schemas canonical and strict

`query_data` and `explain_query` accept the canonical Query Plan object exactly once, with no principal or raw SQL fields. The adapter converts validated tool input into the public plan and delegates it; it never invokes PostgreSQL packages.

### Return neutral runtime data, not driver details

Schema and tabular result structures are serialized into MCP tool content using stable field names. Failures map to safe tool errors/categories. Process diagnostics use stderr so stdout remains parseable MCP traffic.

### Bootstrap before protocol negotiation

The CLI loads Runtime before entering the server loop. A bootstrap error exits non-zero, avoids a half-ready MCP session, and preserves existing Cobra error handling.

## Risks / Trade-offs

- [MCP SDK API/version changes] -> Pin the module version and test an in-process stdio client handshake.
- [Tool JSON drifts from the public contract] -> Test accepted canonical fields and rejected aliases/unknown fields at the tool boundary.
- [Large query results over stdio] -> Existing M2 row/byte limits bound collection; streaming is deferred.

## Migration Plan

This adds a command and dependency without changing existing commands. Rollback removes `mcp stdio` and leaves runtime/M2 core intact.
