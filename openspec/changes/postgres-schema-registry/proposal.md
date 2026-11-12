# Proposal

## Why

M1 валидирует форму запроса и policy grants, но ещё не подтверждает, что разрешённые relation и column существуют в PostgreSQL. Schema Registry нужен как metadata boundary перед будущим execution и как единственный источник безопасной schema discovery.

## What Changes

- Добавить read-only introspection PostgreSQL relation и column metadata для configured datasource.
- Публиковать immutable schema snapshot и проверять по нему source, table и referenced columns.
- Предоставить policy-filtered schema view для trusted principal без раскрытия неразрешённых объектов.
- Добавить Docker-backed integration tests для snapshot, metadata validation и safe failure paths.

## Capabilities

### New Capabilities

- `postgres-schema-registry`: Read-only snapshot PostgreSQL metadata and policy-filtered schema visibility for configured datasources.

### Modified Capabilities

- None.

## Impact

Добавляются schema domain/application service и PostgreSQL metadata adapter поверх существующего DSN configuration. Изменение не выполняет compiled SELECT, не добавляет REST/MCP endpoint, не меняет policies и не вводит schema hot reload.
