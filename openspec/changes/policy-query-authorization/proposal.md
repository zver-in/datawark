# Proposal

## Why

Конфигурация уже описывает select-only grants, но Gateway ещё не применяет их к запросу. Нужен детерминированный policy decision, который не позволит объединить частичные разрешения и будет пригоден для будущих REST, MCP и CLI путей.

## What Changes

- Ввести authorization для предварительно валидного Query Plan на основе configured development principal и YAML policy grants.
- Требовать, чтобы один allow entry целиком покрывал datasource, таблицу, операцию и все колонки из projection, filters и sort.
- Реализовать deny-by-default и машинно-читаемые, нераскрывающие причины allow/deny.
- Добавить dry-run policy explain без SQL compilation или выполнения запроса.

## Capabilities

### New Capabilities

- `query-authorization`: Авторизация ограниченного Query Plan по декларативным select-only policy grants.

### Modified Capabilities

- None.

## Impact

Добавляется policy evaluation/application service и security-focused unit tests. Изменение опирается на `query-plan-validation`, не добавляет transport-адаптеры, DB I/O или SQL compiler.
