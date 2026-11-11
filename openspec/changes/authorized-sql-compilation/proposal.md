# Proposal

## Why

После валидации и authorization Gateway должен преобразовывать разрешённый план в SQL, не возвращаясь к недоверенному пользовательскому вводу. Нужна узкая компиляционная граница, которая исключает SQL injection и не позволяет обойти policy engine.

## What Changes

- Ввести внутренний тип авторизованного Query Plan, создаваемый только после успешного policy decision.
- Компилировать его в один параметризованный PostgreSQL SELECT с безопасной обработкой идентификаторов, filters, sort и limit.
- Запретить compiler API принимать публичный или неавторизованный Query Plan.
- Добавить отрицательные и fuzz/property tests для инъекций, недопустимых идентификаторов и сохранения параметризации.

## Capabilities

### New Capabilities

- `authorized-sql-compilation`: Компиляция авторизованного ограниченного Query Plan в безопасный параметризованный PostgreSQL SELECT.

### Modified Capabilities

- None.

## Impact

Добавляется query/compiler boundary и тесты без соединения с PostgreSQL. Реальное read-only execution, introspection и schema registry остаются в следующем milestone.
