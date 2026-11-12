# Proposal

## Why

M2 реализовал отдельные безопасные компоненты, но бинарник пока не загружает schema snapshots и не предоставляет единый application service для transport adapters. MCP stdio требует immutable runtime boundary, который создаёт все зависимости при старте и fail-closed при ошибке bootstrap.

## What Changes

- Добавить Gateway Runtime, который загружает проверенную конфигурацию и schema snapshot для каждого configured datasource до принятия запросов.
- Публиковать transport-neutral operations для query execution, policy/schema dry-run explanation, allowed datasource listing и policy-filtered schema visibility.
- Гарантировать, что operations используют server-configured development principal и не дают транспорту обращаться к PostgreSQL adapter, policy engine или compiler напрямую.
- Отказывать в запуске при невозможности подготовить полный runtime snapshot; не добавлять hot reload, listener или аутентификацию.

## Capabilities

### New Capabilities

- `gateway-runtime-services`: Immutable bootstrapped runtime services for allowed schema, dry-run explanation, and read-only query operations.

### Modified Capabilities

- None.

## Impact

Добавляются application/runtime packages и wiring из CLI. Используются существующие configuration, schema registry, policy, compiler и read-only executor; синтаксис YAML, PostgreSQL schema и public network API не меняются.
