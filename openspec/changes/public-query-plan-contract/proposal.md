# Proposal

## Why

Публичный Query Plan ещё не имеет единого именованного JSON-контракта: ядро использует `projection` и `order`, тогда как PRD и архитектурный документ показывают другие имена. До появления MCP этот контракт нужно зафиксировать, иначе разные адаптеры смогут принимать несовместимые планы.

## What Changes

- Установить `source`, `table`, `projection`, `filters`, `order` и `limit` как канонические JSON-поля публичного Query Plan v1alpha1.
- **BREAKING**: не принимать документированные, но никогда не реализованные альтернативные поля `select`, `columns`, `orderBy` и `order_by`.
- Описать точную JSON-форму filters, ordering и scalar values, соответствующую существующей strict decoder/validator.
- Привести PRD и архитектурный пример Query Plan к каноническому контракту.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `query-plan-validation`: определить публичное JSON-представление уже валидируемого Query Plan.

## Impact

Изменяются принятый Query Plan specification и документация. Реализация `internal/query` остаётся источником совместимого поведения; последующие MCP и HTTP адаптеры используют этот contract без field aliases.
