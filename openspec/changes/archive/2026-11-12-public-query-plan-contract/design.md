# Design

## Context

`internal/query.Plan` and its strict decoder already implement `projection` and `order`. PRD and ARCH examples use incompatible field names, while no public transport has shipped.

## Goals / Non-Goals

**Goals:**

- Make the existing JSON form the sole public v1alpha1 contract.
- Ensure the future MCP input schema and product documents cannot drift from the decoder.

**Non-Goals:**

- Add aliases, version negotiation, a new Query Plan grammar, or an HTTP endpoint.

## Decisions

### Preserve the implemented field names without aliases

The canonical fields are `source`, `table`, `projection`, `filters`, `order`, and `limit`. Existing strict unknown-field rejection remains the compatibility boundary. Aliases would enlarge the public security surface and create ambiguity without an existing client to preserve.

### Treat scalar and nested forms as part of the contract

The public documentation and future MCP schema will mirror the existing decoder: filter values are JSON scalar values; operators and sort directions use their existing string enums. This keeps transport schemas aligned with the security grammar rather than introducing a translation layer.

### Correct documents rather than code

The M2 decoder is tested and is the factual implementation. PRD and ARCH examples will be updated to it; changing code to a different unshipped naming scheme provides no compatibility benefit.

## Risks / Trade-offs

- [External draft consumers used a document-only alias] -> No released API exists; publish the canonical form before MCP ships.
- [Future transports duplicate schema text] -> Add schema-level tests in the MCP change against the canonical contract.

## Migration Plan

No runtime migration is required. Update documents and public schemas before releasing MCP; rollback restores documentation only because no aliases are accepted.
