ENV_FILE ?= .env
GOOSE_VERSION := v3.24.3
GOOSE := go run github.com/pressly/goose/v3/cmd/goose@$(GOOSE_VERSION)
MIGRATIONS_DIR := backend/migrations

ifneq (,$(wildcard $(ENV_FILE)))
include $(ENV_FILE)
export
endif

.PHONY: db-up db-down db-wait migrate migrate-test backend-run notifications-run test test-integration

db-up:
	docker compose up -d postgres

db-down:
	docker compose down

db-wait:
	docker compose up -d postgres
	until docker compose exec -T postgres pg_isready -U vocab -d vocab_review_dev >/dev/null 2>&1; do sleep 1; done

migrate: db-wait
	test -n "$(DATABASE_URL)"
	$(GOOSE) -dir $(MIGRATIONS_DIR) postgres "$(DATABASE_URL)" up

migrate-test: db-wait
	test -n "$(DATABASE_URL)"
	$(GOOSE) -dir $(MIGRATIONS_DIR) postgres "$(DATABASE_URL)" up

backend-run:
	test -n "$(DATABASE_URL)"
	cd backend && DATABASE_URL="$(DATABASE_URL)" ADDR="$(ADDR)" go run ./cmd/api

notifications-run:
	test -n "$(DATABASE_URL)"
	cd backend && DATABASE_URL="$(DATABASE_URL)" go run ./cmd/notifications

test:
	cd backend && go test ./...

test-integration:
	$(MAKE) ENV_FILE=.env.test migrate-test
	cd backend && set -a && . ../.env.test && set +a && go test ./internal/repository/postgres -count=1
