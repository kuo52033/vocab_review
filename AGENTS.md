# Repository Guidelines

## Agent Tooling
Keep the agent surface small. Use CodeGraph only for structural code questions such as symbol lookup, callers/callees, impact analysis, or focused architecture context. Use `rg`/file reads for literal text, config, docs, and exact strings. Do not add agent-specific plans, generated working notes, or tool integrations unless they are part of the checked-in development workflow.

## Project Structure & Module Organization
This repository is a small monorepo for the Vocab Review product.

- `backend/`: Go API server. Entry point lives in `backend/cmd/api`, core logic in `backend/internal/{httpapi,service,repository,domain,clock}`, the Postgres implementation lives in `backend/internal/repository/postgres`, and SQL migrations live in `backend/migrations`.
- `apps/web/`: React + TypeScript app built with Vite. Source files live in `apps/web/src`.
- `apps/chrome-extension/`: React + TypeScript Chrome extension. Source files live in `apps/chrome-extension/src`, static manifest assets in `public/`, and build output in `dist/`.
- `apps/ios/`: SwiftUI iOS app under `apps/ios/VocabReview`. The active Xcode project is `apps/ios/VocabReview/VocabReview/VocabReview.xcodeproj`, with source files in `apps/ios/VocabReview/VocabReview/VocabReview/{Models,ViewModels,Views}`.
- `deploy/`: production runtime files copied to EC2 by GitHub Actions. Keep this directory small; it should contain deploy-only files such as `docker-compose.prod.yml` and `Caddyfile`, not application source or secrets.
- `.github/workflows/ci.yml`: the single CI/deploy workflow. It runs backend tests, frontend builds, builds/pushes the backend image, and deploys through AWS SSM on pushes to `master`.

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
- `xcodebuild -project apps/ios/VocabReview/VocabReview/VocabReview.xcodeproj -scheme VocabReview -destination 'generic/platform=iOS Simulator' -derivedDataPath .xcode-derived-data build`: verify the iOS app builds.
- `APP_ENV_FILE=../.env.production docker compose --env-file .env.production -f deploy/docker-compose.prod.yml config --quiet`: validate the production Compose file locally with a temporary/safe env file. Do not commit `.env.production`.

## Coding Style & Naming Conventions
Go code should follow `gofmt` defaults and keep packages focused. React and TypeScript files use 2-space indentation, `PascalCase` for components, and `camelCase` for hooks, helpers, and local state. Keep filenames descriptive and lowercase for Go, and match component names for React files when new components are introduced.

SwiftUI screens live in `Views/`, shared iOS visual tokens live in `Views/AppTheme.swift`, app state and API calls live in `ViewModels/SessionStore.swift`, and API DTOs live in `Models/APIModels.swift`. Keep the iOS `Review` tab focused on due cards and the `Library` tab focused on all active cards.

Do not edit generated output in `dist/` or dependencies in `node_modules/`.

## Deployment Workflow
Production deployment is automated in `.github/workflows/ci.yml` and documented in `docs/deployment.md`.

- The workflow triggers only on pushes to `master`.
- The `backend` job runs Go tests, applies migrations to the CI Postgres service, and runs Postgres integration tests.
- The `frontend` job runs `npm ci`, `npm run build:web`, and `npm run build:extension`.
- The `deploy` job waits for `backend` and `frontend`, builds `backend/Dockerfile`, tags the image as `master-${GITHUB_SHA::12}`, pushes it to ECR, packages only `deploy/`, and sends AWS SSM commands to the EC2 instance.
- EC2 must already have `/opt/vocab-review/.env.production`; the workflow only adds or replaces `BACKEND_IMAGE=...`. Never commit `.env.production`, `DATABASE_URL`, AWS credentials, or other production secrets.
- Required GitHub Actions configuration is `AWS_ROLE_TO_ASSUME` as a secret, plus `AWS_REGION`, `AWS_ACCOUNT_ID`, `ECR_REPOSITORY`, `EC2_INSTANCE_ID`, and `DEPLOY_DIR` as variables.
- The production Compose stack uses `${BACKEND_IMAGE}` for the API and migration image, runs migrations with `/app/goose`, serves Caddy on `80`/`443`, and routes `api.vocabreview.uk` to `api:8080`.

## Testing Guidelines
Backend tests use Go’s built-in `testing` package and live beside the code they verify with `_test.go` names, for example `backend/internal/service/review_test.go`. Favor deterministic service tests with hand-written fakes, and keep repository integration coverage pointed at a real Postgres instance plus the checked-in migration chain. There are currently no frontend test scripts configured, so document manual verification steps for web or extension UI changes in the PR.

## Commit & Pull Request Guidelines
The available backend history uses Conventional Commits, for example `feat: add initial backend service`; use the same style across the repo. Keep commits scoped and imperative.

PRs should include a short behavior summary, linked issue or task when available, test evidence (`go test ./...`, relevant builds), and screenshots or screen recordings for UI changes.
