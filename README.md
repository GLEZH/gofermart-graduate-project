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

## Команды

```bash
make build-dev
make migrate
make test
make test-ci
```
