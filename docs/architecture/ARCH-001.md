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
- В начальной версии запросы исполняются от единственного principal, заданного конфигурацией сервера; аутентификация отложена.

### DataWark Assistant

- **Отдельный репозиторий и процесс**: `datawark-assistant`.
- Web UI: React + TypeScript; backend: Go (предварительный выбор).
- Backend вызывает LLM, строит Query Plan и обращается к Gateway через REST API.
- Не содержит DB credentials, SQL Executor и собственного Policy Engine.
- Не является обязательным компонентом Gateway.

## 3. Режимы запуска Gateway

| Режим | Команда | Назначение |
|---|---|---|
| CLI | `datawark config validate`, `datawark policy explain`, `datawark source test` | Локальная конфигурация и диагностика |
| MCP stdio | `datawark mcp stdio` | Локальные MCP-клиенты; без сетевого listener |
| HTTP Server | `datawark serve` | REST API и MCP Streamable HTTP на одном listener |

Административные CLI-команды доступны только доверенному локальному оператору. Удалённый административный API не входит в MVP.

## 4. Компоненты Gateway

| Компонент | Ответственность |
|---|---|
| CLI / REST / MCP adapters | Декодирование запросов и ответы |
| Identity | Server-configured development principal; будущая проверка credentials |
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
Transport → Development Principal
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

**Разрешено в MVP:** одна таблица, явные колонки, простые типизированные фильтры (AND), сортировка, LIMIT.

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
3. Resource limits (`statement_timeout`, лимиты строк и размера результата, контроль конкурентности) отложены до следующего этапа.
4. Неподдерживаемые поля, операторы, типы и ресурсы отклоняются.
5. Schema API возвращает только разрешённые метаданные.
6. Audit отложен до следующего этапа; начальная версия не предназначена для production.
7. В HTTP и MCP stdio используется единственный server-configured development principal; параметры запроса не задают principal или роль.
8. MCP tools не предоставляют административные операции изменения политик и источников.
9. При обновлении политики нужна явная семантика для уже выполняющихся запросов; hot reload в MVP необязателен.
10. Text из базы считается недоверенными данными для Assistant (защита от prompt injection).

## 9. HTTP / MCP контракты

| Интерфейс | Операция | Назначение |
|---|---|---|
| REST | `POST /api/v1/query` | Исполнить Query Plan |
| REST | `GET /api/v1/schema/...` | Получить разрешённую схему |
| REST | `POST /api/v1/query/explain` | Проверить план без исполнения |
| MCP | `datawark_execute_query` | Исполнить Query Plan |
| MCP | `datawark_describe_schema` | Получить разрешённую схему |
| MCP | `datawark_list_sources` | Список разрешённых источников |
| MCP | `datawark_explain_query` | Проверить план без исполнения |

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
├── docs/adr/
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
- DSN передаётся через переменные окружения / secrets, не хранится в Git. API keys будут добавлены вместе с HTTP-аутентификацией.
- Политики загружаются при запуске; некорректная конфигурация блокирует старт. Изменение требует перезапуска; hot reload отсутствует.
- Docker Compose: Gateway + тестовый PostgreSQL; отдельный пример полного стека с Assistant.

## 13. Стратегия тестирования и CI

В проекте поддерживаются два независимых вида тестов.

| Вид | Расположение и запуск | Граница проверки |
|---|---|---|
| Unit | `*_test.go` рядом с проверяемым Go-пакетом; `make test` | Доменная логика, парсинг YAML, policy evaluation, Query Plan и SQL Compiler. Без Docker, сети и настоящей БД. |
| Integration | `tests/integration/`, build tag `integration`; `make test-integration` | Взаимодействие с реальным PostgreSQL: подключение, introspection, read-only execution, DB grants и защита от SQL injection. Перед запуском команда поднимает изолированную PostgreSQL через `tests/integration/compose.yaml`. |

- Unit-тесты обязательны для новой ветви логики; отрицательные проверки policy и компилятора добавляются рядом с позитивными.
- Интеграционные тесты не используют внешние БД и не принимают production credentials. Они получают DSN только через `DATAWARK_TEST_POSTGRES_DSN`, который задаёт тестовая команда.
- Fuzz/property tests для парсера, Query Plan и SQL Compiler выполняются отдельно локально и добавляются в CI по мере появления корпусов.
- Pull request и изменения в `main` запускают GitHub Actions: проверку форматирования (`gofmt`), `go vet`, unit-тесты с race detector и интеграционные тесты с Docker Compose. Интеграционная job всегда очищает Docker volume после завершения.

## 14. Порядок реализации

1. **M0 — Foundation:** CLI, YAML config, bootstrap, domain types.
2. **M1 — Security Core:** Policy Engine, Query Plan, Validator, SQL Compiler, unit/fuzz tests.
3. **M2 — PostgreSQL:** introspection, executor, ограничения, интеграционные тесты.
4. **M3 — MCP stdio:** локальное использование AI-клиентами.
5. **M4 — HTTP Gateway:** REST, MCP HTTP, API keys, аудит, observability.
6. **M5 — Public MVP:** Docker Compose, quickstart, security regression tests.
7. **Отдельный релиз Assistant:** Web UI + Go backend + LLM; только через публичный Gateway API.

## 14. Архитектурные инварианты

- Gateway работает без Assistant и без LLM.
- Assistant не выполняет SQL и не подключается к PostgreSQL напрямую.
- Все транспорты используют единый security pipeline.
- Прямой SQL недоступен через публичные API MVP.
- Доступ отсутствует без явного разрешения.
- Поддержка нескольких СУБД и расширенных политик не усложняет MVP.
