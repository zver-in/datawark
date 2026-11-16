# ARCH-001 — DataWark Architecture

- **Status:** Draft
- **Version:** 0.1
- **Related:** PRD-001
- **Language:** Go
- **Architecture:** Hybrid, CLI-first, modular monolith

## 1. Назначение

**DataWark Gateway** — open-source, self-hosted сервис безопасного доступа AI-агентов и приложений к структурированным данным. Все запросы проходят серверную авторизацию на основе декларативных YAML-политик.

**DataWark Assistant** — отдельное Web-приложение с чатом и LLM, первый клиент Gateway. Assistant не подключается к БД напрямую и не входит в поставку Gateway.

## 2. Состав системы

```text
AI Agents ─────────── MCP stdio / MCP HTTP ─┐
Custom Applications ───────────── REST ──────┼──> DataWark Gateway (Go) ──> PostgreSQL
DataWark Assistant (Web + Backend) ─ REST ──┘
```

### DataWark Gateway

- Один Go-бинарник `datawark`.
- Headless: **без Web UI и без зависимости от LLM**.
- Интерфейсы: CLI, MCP stdio, MCP Streamable HTTP, REST API.
- В MVP: PostgreSQL и операции чтения через ограниченный Structured Query Plan.
- Единый механизм policy enforcement для всех интерфейсов.
- MCP stdio uses a server-configured local principal; network REST and MCP authenticate API-key identities.

### DataWark Assistant

- **Отдельный репозиторий и процесс**: `datawark-assistant`.
- Web UI: React + TypeScript; backend: Go (предварительный выбор).
- Backend вызывает LLM, строит Query Plan и обращается к Gateway через REST API.
- Не содержит DB credentials, SQL Executor и собственного Policy Engine.
- Не является обязательным компонентом Gateway.

## 3. Режимы запуска Gateway

| Режим | Команда | Назначение | Состояние |
|---|---|---|---|
| CLI | `datawark config validate`, `datawark source test` | Локальная конфигурация и диагностика | Реализовано |
| CLI explain | `datawark policy explain` | Локальный административный dry-run | Реализовано |
| MCP stdio | `datawark mcp stdio` | Локальные MCP-клиенты; без сетевого listener | Реализовано |
| HTTP Server | `datawark serve` | REST API и MCP Streamable HTTP на одном listener | Реализовано |

Административные CLI-команды доступны только доверенному локальному оператору. Удалённый административный API не входит в MVP.

## 4. Компоненты Gateway

| Компонент | Ответственность |
|---|---|
| CLI / REST / MCP adapters | Декодирование запросов и ответы |
| Identity | Server-configured development principal для локального режима; API keys для Public Gateway MVP; OIDC позже |
| Application Services | Оркестрация запросов, схемы, политик и диагностики |
| Schema Registry | Снимок метаданных БД и фильтрация видимой клиенту схемы |
| Query Validator | Проверка структуры, типов, операторов и ресурсов Query Plan |
| Policy Engine | Deny-by-default; проверка разрешений |
| SQL Compiler | Генерация параметризованного PostgreSQL SQL из авторизованного плана |
| PostgreSQL Adapter | Schema introspection, пул соединений, read-only выполнение |
| Runtime / Bootstrap | Загрузка конфигурации, сборка зависимостей, lifecycle, shutdown |

CLI, REST и MCP обращаются к **одним Application Services**. Прямой доступ транспортов к компилятору и исполнителю запрещён.

## 5. Путь выполнения запроса

```text
Transport → Trusted Principal (development principal или API key identity)
          → Runtime Snapshot (policy + schema)
          → Decode / Validate Query Plan
          → Policy Evaluation (allow / deny)
          → SQL Compilation
          → PostgreSQL Execution
          → Response
```

- Отказ на любом этапе до исполнения не приводит к SQL-запросу.
- SQL Compiler получает только проверенный и авторизованный внутренний тип запроса.
- Версии политики и схемы фиксируются на время обработки запроса.

## 6. Query Plan (MVP)

Публичные API **не принимают произвольный SQL**.

```json
{
  "source": "analytics",
  "table": "public.orders",
  "projection": ["id", "total", "created_at"],
  "filters": [
    {"column": "status", "operator": "eq", "value": "paid"}
  ],
  "order": [
    {"column": "created_at", "direction": "desc"}
  ],
  "limit": 10
}
```

**Разрешено в MVP:** одна таблица, явные колонки, простые типизированные фильтры (AND), сортировка, обязательный положительный LIMIT.

**Не входит в MVP:** произвольный SQL, запись, JOIN, GROUP BY, SQL-функции, подзапросы, агрегаты и произвольные выражения.

Колонки из `projection`, `filters` и `order` тоже проверяются политикой. Идентификаторы берутся из валидированной схемы и безопасно экранируются; значения передаются параметрами. Публичный Query Plan использует только поля `source`, `table`, `projection`, `filters`, `order` и `limit`; алиасы не принимаются. Значение filter — JSON scalar (string, number, boolean или `null`), а `direction` — `asc` или `desc`.

## 7. Политики доступа

- YAML Policy-as-Code; строгий парсинг и валидация при загрузке.
- `deny-by-default` — неизменяемый инвариант, не конфигурационный переключатель.
- Разрешения: `principal` / роль → источник → таблица → колонки → операция.
- Каждый grant проверяется как единое целое; нельзя комбинировать разрешённую таблицу из одного grant с колонками из другого.
- `Policy Explain` возвращает машинно-читаемый код решения. Детали о скрытых ресурсах не раскрываются обычным клиентам.
- Row-level policies, masking и сложные условия — вне MVP.

## 8. Безопасность

1. LLM и Assistant не получают DB credentials; в начальной версии Gateway использует единственный principal, заданный серверной конфигурацией.
2. PostgreSQL использует отдельного пользователя с минимальными правами; запросы выполняются в read-only транзакциях.
3. Resource limits (`statement_timeout`, лимиты строк и размера результата, контроль конкурентности) реализованы и применяются на PostgreSQL execution path.
4. Неподдерживаемые поля, операторы, типы и ресурсы отклоняются.
5. Schema API возвращает только разрешённые метаданные.
6. Public Gateway MVP пишет структурированные JSON audit events. Гарантии формирования и передачи событий внешнему сборщику должны быть определены до реализации; metadata database, внутренняя очередь и transactional outbox в MVP не добавляются.
7. MCP stdio использует server-configured development principal. HTTP-аутентификация использует API keys; параметры Query Plan не задают principal или роль. OIDC добавляется после MVP.
8. MCP tools не предоставляют административные операции изменения политик и источников.
9. При обновлении политики нужна явная семантика для уже выполняющихся запросов; hot reload в MVP необязателен.
10. Text из базы считается недоверенными данными для Assistant (защита от prompt injection).

## 9. HTTP / MCP контракты

| Интерфейс | Операция | Назначение |
|---|---|---|
| REST | `POST /api/v1/query` | Исполнить Query Plan |
| REST | `GET /api/v1/schema/...` | Получить разрешённую схему |
| REST | `POST /api/v1/query/explain` | Проверить план без исполнения |
| MCP | `query_data` | Исполнить Query Plan |
| MCP | `list_allowed_schema` | Получить разрешённые источники и отфильтрованную схему |
| MCP | `explain_query` | Проверить план без исполнения |

MCP HTTP endpoint: `/mcp`. REST и MCP используют общий `QueryService` и одинаковые policy decisions. Их Query Plan input schema использует канонические поля `source`, `table`, `projection`, `filters`, `order` и `limit`, соответствующие decoder; точные схемы запросов и ответов фиксируются в OpenAPI и MCP tool schemas.

## 10. Технологический стек

| Область | Выбор |
|---|---|
| CLI | Cobra |
| HTTP | `net/http` + `chi/v5` |
| MCP | `modelcontextprotocol/go-sdk` |
| PostgreSQL | `pgx/v5` + `pgxpool` |
| YAML | `go.yaml.in/yaml/v3` |
| SQL Compiler | Собственный компилятор ограниченного Query Plan, без ORM |
| Policy Engine | Собственная типизированная модель; CEL/OPA не нужны в MVP |
| Logging | `log/slog` (JSON) |
| Metrics / Tracing | Prometheus / OpenTelemetry |
| Tests | `testing`, `testify`, Go fuzzing, Docker Compose для интеграционных тестов |
| Distribution | Go binary, Docker, Docker Compose |

## 11. Структура репозитория

```text
datawark/
├── cmd/datawark/
├── internal/
│   ├── cli/
│   ├── bootstrap/
│   ├── app/
│   ├── identity/
│   ├── policy/
│   ├── query/
│   ├── schema/
│   ├── config/
│   └── adapters/
│       ├── postgres/
│       ├── rest/
│       ├── mcp/
├── api/openapi.yaml
├── configs/examples/
├── tests/{integration,security}/
├── Dockerfile
└── compose.yaml
```

Отдельный репозиторий `datawark-assistant/` содержит `backend/` и `frontend/`.

## 12. Поставка и конфигурация

- Один бинарник Gateway; отдельная поставка Assistant.
- Единственный YAML-файл использует `version: v1alpha1` и содержит `server`, `datasources` и `policies`.
- `server.developmentPrincipal` — единственный principal начальной версии. Он задаётся конфигурацией сервера, а не HTTP- или MCP-запросом.
- Каждый datasource содержит уникальное `name`, `driver: postgres` и `dsnEnv`. `dsnEnv` — имя переменной окружения с DSN PostgreSQL, а не сам DSN.
- Каждый policy содержит уникальное `name`, непустые `principals`, `datasource` и `allow`. `allow` содержит `table`, `operations: [select]` и разрешённые `columns`.
- YAML-парсер отклоняет неизвестные поля, повторяющиеся имена, пустые обязательные значения и неподдерживаемые операции/драйверы.
- Для допуска запроса один элемент `allow` обязан целиком покрыть источник, таблицу и все колонки из `projection`, `filters` и `order`; разрешения из разных элементов не объединяются.
- DSN и API keys передаются через переменные окружения / secrets, не хранятся в Git.
- Gateway не использует отдельную metadata database: конфигурация и политики загружаются из YAML, а schema snapshots находятся в памяти процесса.
- Политики загружаются при запуске; некорректная конфигурация блокирует старт. Изменение требует перезапуска; hot reload отсутствует.
- Docker Compose: loopback-only Gateway + PostgreSQL demo with a least-privilege reader role. Assistant is not shipped.

## 13. Стратегия тестирования и CI

В проекте поддерживаются два независимых вида тестов.

| Вид | Расположение и запуск | Граница проверки |
|---|---|---|
| Unit | `*_test.go` рядом с проверяемым Go-пакетом; `make test` | Доменная логика, парсинг YAML, policy evaluation, Query Plan и SQL Compiler. Без Docker, сети и настоящей БД. |
| Integration | `tests/integration/`, build tag `integration`; `make test-integration` | Взаимодействие с реальным PostgreSQL: подключение, introspection, read-only execution, DB grants и защита от SQL injection. Перед запуском команда поднимает изолированную PostgreSQL через `tests/integration/compose.yaml`. |

- Unit-тесты обязательны для новой ветви логики; отрицательные проверки policy и компилятора добавляются рядом с позитивными.
- Интеграционные тесты не используют внешние БД и не принимают production credentials. Они получают DSN только через `DATAWARK_TEST_POSTGRES_DSN`, который задаёт тестовая команда.
- Fuzz/property tests для парсера, Query Plan и SQL Compiler выполняются отдельно локально и добавляются в CI по мере появления корпусов.
- Pull request и изменения в `main` запускают один GitHub Actions workflow: format/vet, race unit tests, PostgreSQL integration tests и `make verify-public-mvp`.

## 14. Порядок реализации

1. **M0 — Foundation:** CLI, YAML config, bootstrap, domain types.
2. **M1 — Security Core:** Policy Engine, Query Plan, Validator, SQL Compiler, unit/fuzz tests.
3. **M2 — PostgreSQL:** introspection, executor, ограничения, интеграционные тесты.
4. **M3 — MCP stdio:** локальное использование AI-клиентами.
5. **M4 — HTTP Gateway (complete):** REST, MCP Streamable HTTP, API keys, JSON-аудит, observability и CLI `policy explain`.
6. **M5 — Public Gateway release:** Docker Compose, quickstart, threat model, security regressions and release gate.
7. **M6 — AI Database Assistant:** Web UI + Go backend + LLM; только после Public Gateway MVP и только через публичный Gateway API.
8. **После M6 — Advanced governance:** OIDC, row-level policies, masking и дополнительные СУБД.

## 14. Архитектурные инварианты

- Gateway работает без Assistant и без LLM.
- Assistant не выполняет SQL и не подключается к PostgreSQL напрямую.
- Все транспорты используют единый security pipeline.
- Прямой SQL недоступен через публичные API MVP.
- Доступ отсутствует без явного разрешения.
- Поддержка нескольких СУБД и расширенных политик не усложняет MVP.
