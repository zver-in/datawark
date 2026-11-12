# Proposal

## Why

После M1 и Schema Registry Gateway способен безопасно сформировать SQL, но ещё не может вернуть данные. Нужен единый execution path, который допускает к PostgreSQL только полностью проверенный запрос и выполняет его в read-only транзакции.

## What Changes

- Добавить application QueryService, последовательно объединяющий validation, policy authorization, schema validation, compilation и execution.
- Выполнять только compiled PostgreSQL SELECT через configured datasource и explicit read-only transaction.
- Возвращать column-labelled rows и безопасные execution failures без SQL/DSN/credential leakage.
- Добавить Docker-backed integration tests для успешного чтения, policy/schema rejection и database write protection.

## Capabilities

### New Capabilities

- `read-only-query-execution`: Выполнение полностью проверенных Query Plans против PostgreSQL и возврат read-only results.

### Modified Capabilities

- None.

## Impact

Добавляются application service и PostgreSQL executor поверх `postgres-schema-registry`, `query-authorization` и `authorized-sql-compilation`. REST/MCP transport, runtime resource limits и authentication остаются вне scope.
