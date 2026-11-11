# Proposal

## Why

Gateway уже принимает и строго валидирует конфигурацию, но ещё не имеет безопасного контракта данных для клиентских запросов. До policy enforcement и SQL compilation нужен ограниченный, однозначно валидируемый Query Plan вместо произвольного SQL.

## What Changes

- Ввести публичную типизированную модель Query Plan для одного PostgreSQL-источника и одной таблицы.
- Валидировать обязательные поля, проекцию, AND-фильтры, сортировку, limit, поддерживаемые операторы и типы значений.
- Отклонять произвольный SQL, JOIN, агрегаты, подзапросы, выражения и неподдерживаемые поля до authorization.
- Предоставить структурированные ошибки валидации, безопасные для передачи transport-адаптерам.

## Capabilities

### New Capabilities

- `query-plan-validation`: Приём и строгая проверка ограниченного структурированного плана read-only запроса.

### Modified Capabilities

- None.

## Impact

Добавляется доменный пакет Query Plan и его unit/fuzz tests. Изменение не добавляет HTTP/MCP transport, policy decision, schema introspection или доступ к PostgreSQL.
