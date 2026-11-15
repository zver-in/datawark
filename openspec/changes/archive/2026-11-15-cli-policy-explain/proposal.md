# Proposal

## Why

Operators need a local way to inspect the Gateway decision for a canonical Query Plan without executing SQL or starting a network listener. The runtime already supports safe explanation, but no CLI adapter exposes it.

## What Changes

- Add `datawark policy explain --config <path>` with a Query Plan read from a required file or stdin.
- Decode the canonical JSON contract and evaluate it through the complete runtime explanation path without compilation or execution.
- Use the configured development principal and do not accept client-selected identity in this milestone.
- Return a stable machine-readable result and safe non-zero failures without revealing hidden resources or secrets.
- Add command and integration coverage proving no execution transaction is opened.

## Capabilities

### New Capabilities

- `cli-policy-explain`: Local trusted dry-run of a canonical Query Plan through Gateway validation, authorization, and metadata checks.

### Modified Capabilities

- None.

## Impact

This adds a Cobra command and transport mapping over the principal-aware runtime. It depends on `trusted-principal-runtime` but does not block the HTTP changes.
