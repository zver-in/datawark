# Proposal

## Why

Read-only execution ограничивает тип SQL, но сам по себе не ограничивает время, объём результата или число одновременных запросов. Runtime guardrails нужны, чтобы допустимый запрос не мог истощить Gateway или PostgreSQL.

## What Changes

- Добавить совместимые optional runtime limits для PostgreSQL: statement timeout, максимум строк, байтов результата и одновременных executions.
- Применять ограничения server-side к каждому execution и прерывать запросы, превысившие их.
- Возвращать стабильные безопасные error codes при timeout, capacity exhaustion и result limit.
- Добавить integration/security tests для каждого ограничения и очистки ресурсов после отказа.

## Capabilities

### New Capabilities

- `postgres-runtime-guardrails`: Server-enforced time, capacity, and response-size bounds for PostgreSQL read execution.

### Modified Capabilities

- `gateway-configuration`: Расширить строгую versioned configuration совместимыми optional runtime limit settings.

## Impact

Изменяются config model/validation и PostgreSQL execution path после `read-only-query-execution`. Нет изменений policy grants, Query Plan grammar, API transport или DB schema.
