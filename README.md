# Gophermart

Накопительная система лояльности с HTTP API, PostgreSQL и фоновой проверкой начислений.

## Запуск

```bash
make local
make run
```

Полный стек в Docker:

```bash
docker compose --env-file .env.dev up --build
```

## Конфигурация

`DATABASE_URI` можно не задавать: строка подключения собирается из `DB_HOST` (по умолчанию
`localhost`), `DB_PORT`, `DB_USER`, `DB_PASSWORD`, `DB_TABLE_NAME` и `DB_SSLMODE`
(по умолчанию `disable`). Явный `DATABASE_URI` или флаг `-d` имеют приоритет над частями.

## Команды

```bash
make build-dev
make migrate
make test
make test-ci
```

Тесты БД идут в схему `gophermart_test` той же базы, которую поднимает `make local`,
и пропускаются, если `TEST_DATABASE_URI` не задан.
