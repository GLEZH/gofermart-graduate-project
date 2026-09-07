include .env.dev

COVERAGE_FILE = coverage.out

DB_HOST ?= localhost
DB_SSLMODE ?= disable
DATABASE_URI ?= postgres://$(DB_USER):$(DB_PASSWORD)@$(DB_HOST):$(DB_PORT)/$(DB_TABLE_NAME)?sslmode=$(DB_SSLMODE)
TEST_DATABASE_URI ?= $(DATABASE_URI)

export RUN_ADDRESS
export DB_HOST
export DB_PORT
export DB_USER
export DB_PASSWORD
export DB_TABLE_NAME
export DB_SSLMODE
export DATABASE_URI
export TEST_DATABASE_URI
export ACCRUAL_SYSTEM_ADDRESS
export AUTH_SECRET
export ACCRUAL_POLL_INTERVAL

develop:
	go mod download

build-dev:
	mkdir -p bin
	go build -o ./bin/gophermart ./cmd/gophermart

local:
	docker compose --env-file .env.dev -f docker-compose.dev.yml up --force-recreate --build

local-down:
	docker compose --env-file .env.dev -f docker-compose.dev.yml down

migrate:
	go run ./cmd/migrate -d "$(DATABASE_URI)" up

test:
	go test -race -count=1 ./...

test-ci:
	go test -race -coverpkg=./internal/... -coverprofile=$(COVERAGE_FILE) -covermode=atomic -count=1 ./...
	go tool cover -func=$(COVERAGE_FILE) | tail -n 1
	@coverage=$$(go tool cover -func=$(COVERAGE_FILE) | awk '/^total:/ {gsub("%", "", $$3); print $$3}'); awk -v coverage="$$coverage" 'BEGIN {if (coverage < 80) {printf "coverage %.1f%% is below 80%%\n", coverage; exit 1}}'

run:
	go run ./cmd/gophermart
