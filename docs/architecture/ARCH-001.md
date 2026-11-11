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
DataWark Assistant (Web + Backend) ─ REST ──┘            │
                                                        └──> Audit sink
```

### DataWark Gateway

- Один Go-бинарник `datawark`.
- Headless: **без Web UI и без зависимости от LLM**.
- Интерфейсы: CLI, MCP stdio, MCP Streamable HTTP, REST API.
- В MVP: PostgreSQL и операции чтения через ограниченный Structured Query Plan.
- Единый механизм авторизации для всех интерфейсов.

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
| CLI / REST / MCP adapters | Декодирование запросов, ответы, transport-specific auth |
| Identity | Проверка credentials, создание доверенного `Actor` / `Principal` |
| Application Services | Оркестрация запросов, схемы, политик и диагностики |
| Schema Registry | Снимок метаданных БД и фильтрация видимой клиенту схемы |
| Query Validator | Проверка структуры, типов, операторов и ресурсов Query Plan |
| Policy Engine | Deny-by-default; проверка разрешений и вычисление лимитов |
| SQL Compiler | Генерация параметризованного PostgreSQL SQL из авторизованного плана |
| PostgreSQL Adapter | Schema introspection, пул соединений, read-only выполнение |
| Audit | Фиксация решений авторизации и результатов исполнения |
| Runtime / Bootstrap | Загрузка конфигурации, сборка зависимостей, lifecycle, shutdown |

CLI, REST и MCP обращаются к **одним Application Services**. Прямой доступ транспортов к компилятору и исполнителю запрещён.

## 5. Путь выполнения запроса

```text
Transport → Authentication / Actor
          → Runtime Snapshot (policy + schema)
          → Decode / Validate Query Plan
          → Policy Evaluation (allow / deny + limits)
          → SQL Compilation
          → Audit admission
          → PostgreSQL Execution
          → Audit completion → Response
```

- Отказ на любом этапе до исполнения не приводит к SQL-запросу.
- Audit фиксирует и отказы, и завершение запросов; обязательная запись до исполнения должна иметь определённую fail-closed семантику.
- SQL Compiler получает только проверенный и авторизованный внутренний тип запроса.
- Версии политики и схемы фиксируются на время обработки запроса.

## 6. Query Plan (MVP)

Публичные API **не принимают произвольный SQL**.

```json
{
  "source": "analytics",
  "table": "public.orders",
  "columns": ["id", "total", "created_at"],
  "filters": [
    {"column": "status", "operator": "eq", "value": "paid"}
  ],
  "order_by": [
    {"column": "created_at", "direction": "desc"}
  ],
  "limit": 10
}
```

**Разрешено в MVP:** одна таблица, явные колонки, простые типизированные фильтры (AND), сортировка, LIMIT.

**Не входит в MVP:** произвольный SQL, запись, JOIN, GROUP BY, SQL-функции, подзапросы, агрегаты и произвольные выражения.

Колонки, используемые в `filters` и `order_by`, тоже проверяются политикой. Идентификаторы берутся из валидированной схемы и безопасно экранируются; значения передаются параметрами.

## 7. Политики доступа

- YAML Policy-as-Code; строгий парсинг и валидация при загрузке.
- `deny-by-default` — неизменяемый инвариант, не конфигурационный переключатель.
- Разрешения: `principal` / роль → источник → таблица → колонки → операция.
- Ограничения: максимум строк, байт ответа, времени выполнения и параллельных запросов.
- Каждый grant проверяется как единое целое; нельзя комбинировать разрешённую таблицу из одного grant с колонками из другого.
- `Policy Explain` возвращает машинно-читаемый код решения. Детали о скрытых ресурсах не раскрываются обычным клиентам.
- Row-level policies, masking и сложные условия — вне MVP.

## 8. Безопасность

1. LLM и Assistant не получают DB credentials; аутентификация и роли определяются Gateway.
2. PostgreSQL использует отдельного пользователя с минимальными правами; запросы выполняются в read-only транзакциях.
3. В Executor применяются `statement_timeout`, context deadline, лимиты строк и размера результата, контроль конкурентности.
4. Неподдерживаемые поля, операторы, типы и ресурсы отклоняются.
5. Schema API возвращает только разрешённые метаданные.
6. Audit не хранит строки результатов и секреты по умолчанию.
7. В MCP stdio principal закрепляется доверенным локальным запуском; параметр principal не считается аутентификацией. В HTTP каждый запрос проходит проверку credentials.
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

MCP HTTP endpoint: `/mcp`. REST и MCP используют общий `QueryService` и одинаковые policy decisions. Точные схемы запросов и ответов фиксируются в OpenAPI и MCP tool schemas.

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
| Tests | `testing`, `testify`, Go fuzzing, `testcontainers-go` |
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
│   ├── audit/
│   ├── config/
│   └── adapters/
│       ├── postgres/
│       ├── rest/
│       ├── mcp/
│       └── auditjsonl/
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
- YAML: источники, principals, grants, лимиты, transport settings.
- DSN и API keys передаются через переменные окружения / secrets, не хранятся в Git.
- В MVP политики загружаются при запуске; некорректная конфигурация блокирует старт.
- Docker Compose: Gateway + тестовый PostgreSQL; отдельный пример полного стека с Assistant.

## 13. Порядок реализации

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
