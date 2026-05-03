# Repository Guidelines

## Project Structure & Module Organization
This repository is a Go backend module: `vocabreview/backend`.

- `cmd/api/` contains the executable entrypoint for the HTTP server.
- `internal/httpapi/` holds route registration, middleware, and request/response handlers.
- `internal/service/` contains application logic such as review scheduling and business rules.
- `internal/repository/` defines storage contracts and shared repository types.
- `internal/repository/postgres/` implements the persistence layer with `pgx`.
- `internal/domain/` defines shared domain models.
- `internal/clock/` isolates time access for deterministic tests.
- `migrations/` contains versioned SQL schema changes for PostgreSQL.

Keep new code inside `internal/` unless it is a binary entrypoint under `cmd/`.

## Build, Test, and Development Commands
- `make db-up`: start the local PostgreSQL container from the repo root.
- `make migrate`: apply SQL migrations to the development database from the repo root.
- `go run ./cmd/api`: start the API locally on `:8080` with `DATABASE_URL` set.
- `go test ./...`: run the full test suite.
- `go test ./internal/service -run Review`: run a focused subset of review tests.
- `go test ./internal/repository/postgres -count=1`: run Postgres repository integration tests when `DATABASE_URL` points at the test database.
- `gofmt -w cmd internal`: format all Go source files before committing.

Useful environment variables:

- `ADDR=:8081 go run ./cmd/api` changes the listen address.
- `DATABASE_URL=postgres://... go run ./cmd/api` points the backend at a specific Postgres database.

## Coding Style & Naming Conventions
Use standard Go formatting with tabs via `gofmt`. Prefer small packages with explicit responsibilities matching the current layout.

- Exported identifiers use `CamelCase`.
- Unexported helpers use `camelCase`.
- File names stay lowercase and descriptive, such as `review.go` or `capture_notification.go`.
- Keep handler, service, and repository concerns separated rather than mixing HTTP and business logic.

## Testing Guidelines
Tests use Go’s built-in `testing` package. Place tests next to the code they verify using the `_test.go` suffix, as in `internal/service/review_test.go`.

Favor table-driven tests for review logic and deterministic time-based behavior by injecting hand-written fakes. Repository integration tests should run against a real Postgres instance with the checked-in migration files. Run `go test ./...` before opening a PR.

## Commit & Pull Request Guidelines
Current history uses Conventional Commits, for example: `feat: add initial backend service`. Continue using prefixes like `feat:`, `fix:`, and `test:`.

PRs should include:

- a short description of behavior changes
- linked issue or task reference when available
- test evidence (`go test ./...`)
- sample request/response notes for API changes

## Security & Configuration Tips
Do not commit secrets into `.env` files. Prefer environment variables for runtime configuration and keep schema changes in reviewed migration files.
