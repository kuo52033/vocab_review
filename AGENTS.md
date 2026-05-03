# Repository Guidelines

## Project Structure & Module Organization
This repository is a small monorepo for the Vocab Review product.

- `backend/`: Go API server. Entry point lives in `backend/cmd/api`, core logic in `backend/internal/{httpapi,service,repository,domain,clock}`, the Postgres implementation lives in `backend/internal/repository/postgres`, and SQL migrations live in `backend/migrations`.
- `apps/web/`: React + TypeScript app built with Vite. Source files live in `apps/web/src`.
- `apps/chrome-extension/`: React + TypeScript Chrome extension. Source files live in `apps/chrome-extension/src`, static manifest assets in `public/`, and build output in `dist/`.
- `apps/ios/`: SwiftUI iOS shell under `apps/ios/VocabReview`.

Prefer changes inside the existing layer for each feature rather than mixing HTTP, business, and persistence concerns.

## Build, Test, and Development Commands
- `make db-up`: start the local Postgres container.
- `make migrate`: apply SQL migrations to the development database.
- `make backend-run`: start the API locally on `:8080`.
- `go test ./...` from `backend/`: run the Go test suite.
- `make test-integration`: run the Postgres repository integration test path against the test database.
- `npm run dev:web` from the repo root: start the web app through the workspace.
- `npm run build:web` from the repo root: create the production web bundle.
- `npm run build:extension` from the repo root: build the Chrome extension into `apps/chrome-extension/dist`.
- `gofmt -w cmd internal` from `backend/`: format Go sources before committing.

## Coding Style & Naming Conventions
Go code should follow `gofmt` defaults and keep packages focused. React and TypeScript files use 2-space indentation, `PascalCase` for components, and `camelCase` for hooks, helpers, and local state. Keep filenames descriptive and lowercase for Go, and match component names for React files when new components are introduced.

Do not edit generated output in `dist/` or dependencies in `node_modules/`.

## Testing Guidelines
Backend tests use Go’s built-in `testing` package and live beside the code they verify with `_test.go` names, for example `backend/internal/service/review_test.go`. Favor deterministic service tests with hand-written fakes, and keep repository integration coverage pointed at a real Postgres instance plus the checked-in migration chain. There are currently no frontend test scripts configured, so document manual verification steps for web or extension UI changes in the PR.

## Commit & Pull Request Guidelines
The available backend history uses Conventional Commits, for example `feat: add initial backend service`; use the same style across the repo. Keep commits scoped and imperative.

PRs should include a short behavior summary, linked issue or task when available, test evidence (`go test ./...`, relevant builds), and screenshots or screen recordings for UI changes.
