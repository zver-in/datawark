# Proposal

## Why

После M2 Gateway имеет защищённое внутреннее ядро, но AI-клиент не может использовать его. MCP stdio — первый локальный transport, который делает policy-enforced data access доступным без HTTP listener и без новой identity boundary.

## What Changes

- Добавить команду `datawark mcp stdio --config <path>`, запускающую MCP server поверх Gateway Runtime.
- Предоставить tools `list_allowed_schema`, `query_data` и `explain_query` с каноническим Query Plan contract.
- Направить каждую MCP operation только через runtime services; tool input не может задавать principal, policy, DSN или raw SQL.
- Возвращать безопасные structured results/errors и сохранять stdout исключительно для MCP protocol messages.
- Добавить protocol-level and Docker-backed integration coverage для allowed, denied and malformed requests.

## Capabilities

### New Capabilities

- `mcp-stdio-gateway`: Local MCP stdio tools for policy-enforced schema discovery, query execution, and dry-run explanation.

### Modified Capabilities

- None.

## Impact

Добавляются MCP SDK dependency, CLI command и MCP adapter. Change зависит от `public-query-plan-contract` и `gateway-runtime-services`; REST, MCP HTTP, API keys, audit и observability остаются вне scope.
