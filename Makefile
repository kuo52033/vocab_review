ENV_FILE ?= .env
GOOSE_VERSION := v3.24.3
GOOSE := go run github.com/pressly/goose/v3/cmd/goose@$(GOOSE_VERSION)
MIGRATIONS_DIR := backend/migrations

ifneq (,$(wildcard $(ENV_FILE)))
include $(ENV_FILE)
export
endif

.PHONY: db-up db-down db-wait db-reset db-reset-test migrate migrate-test backend-run notifications-run audio-worker-run backfill-audio test test-integration

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

db-reset: db-wait
	docker compose exec -T postgres psql -U vocab -d vocab_review_dev -v ON_ERROR_STOP=1 -c 'DROP SCHEMA public CASCADE; CREATE SCHEMA public;'
	$(MAKE) migrate

db-reset-test: db-wait
	docker compose exec -T postgres psql -U vocab -d vocab_review_test -v ON_ERROR_STOP=1 -c 'DROP SCHEMA public CASCADE; CREATE SCHEMA public;'
	$(MAKE) ENV_FILE=.env.test migrate-test

backend-run:
	test -n "$(DATABASE_URL)"
	cd backend && DATABASE_URL="$(DATABASE_URL)" ADDR="$(ADDR)" go run ./cmd/api

notifications-run:
	test -n "$(DATABASE_URL)"
	cd backend && DATABASE_URL="$(DATABASE_URL)" go run ./cmd/notifications

audio-worker-run:
	test -n "$(DATABASE_URL)"
	cd backend && DATABASE_URL="$(DATABASE_URL)" go run ./cmd/audio-worker

backfill-audio:
	test -n "$(DATABASE_URL)"
	cd backend && DATABASE_URL="$(DATABASE_URL)" go run ./cmd/backfill-audio

test:
	cd backend && go test ./...

test-integration:
	$(MAKE) ENV_FILE=.env.test migrate-test
	cd backend && set -a && . ../.env.test && set +a && go test ./internal/repository/postgres -count=1
