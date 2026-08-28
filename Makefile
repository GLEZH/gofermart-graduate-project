include .env.dev

COVERAGE_FILE = coverage.out

export RUN_ADDRESS
export DATABASE_URI
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
	DATABASE_URI="$(DATABASE_URI)" go run ./cmd/gophermart
