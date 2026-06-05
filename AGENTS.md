# Repository Guidelines

## Agent Tooling
Keep the agent surface small. Use CodeGraph only for structural code questions such as symbol lookup, callers/callees, impact analysis, or focused architecture context. Use `rg`/file reads for literal text, config, docs, and exact strings. Do not add agent-specific plans, generated working notes, or tool integrations unless they are part of the checked-in development workflow.

## Project Structure & Module Organization
This repository is a small monorepo for the Vocab Review product.

- `backend/`: Go API server. Entry point lives in `backend/cmd/api`, core logic in `backend/internal/{httpapi,service,repository,domain,clock}`, the Postgres implementation lives in `backend/internal/repository/postgres`, and SQL migrations live in `backend/migrations`.
- `apps/web/`: React + TypeScript app built with Vite. Source files live in `apps/web/src`.
- `apps/chrome-extension/`: React + TypeScript Chrome extension. Source files live in `apps/chrome-extension/src`, static manifest assets in `public/`, build output in `dist/`, and Chrome Web Store zip output in `release/`.
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
- `go run ./cmd/audio-worker -once` from `backend/`: process one queued TTS audio batch locally when audio env vars are configured.
- `go run ./cmd/backfill-audio -dry-run -limit 100` from `backend/`: inspect missing-audio cards without writing jobs; omit `-dry-run` to enqueue/attach.
- `go run ./cmd/fixture-data` from `backend/`: seed local development fixture data. This command is for development only and is intentionally not included in the production Docker image.
- `npm run dev:web` from the repo root: start the web app through the workspace.
- `npm run build:web` from the repo root: create the production web bundle.
- `npm run build:extension` from the repo root: build the Chrome extension into `apps/chrome-extension/dist`.
- `npm run release:extension` from the repo root: build the production Chrome extension against `https://api.vocabreview.uk` and package `apps/chrome-extension/release/vocab-review-capture.zip`.
- `gofmt -w cmd internal` from `backend/`: format Go sources before committing.
- `xcodebuild -project apps/ios/VocabReview/VocabReview/VocabReview.xcodeproj -scheme VocabReview -destination 'generic/platform=iOS Simulator' -derivedDataPath .xcode-derived-data build`: verify the iOS app builds.
- `APP_ENV_FILE=../.env.production docker compose --env-file .env.production -f deploy/docker-compose.prod.yml config --quiet`: validate the production Compose file locally with a temporary/safe env file. Do not commit `.env.production`.

## Coding Style & Naming Conventions
Go code should follow `gofmt` defaults and keep packages focused. React and TypeScript files use 2-space indentation, `PascalCase` for components, and `camelCase` for hooks, helpers, and local state. Keep filenames descriptive and lowercase for Go, and match component names for React files when new components are introduced.

SwiftUI screens live in `Views/`, shared iOS visual tokens live in `Views/AppTheme.swift`, app state and API calls live in `ViewModels/SessionStore.swift`, and API DTOs live in `Models/APIModels.swift`. Keep the iOS `Review` tab focused on due cards and the `Library` tab focused on all active cards.

Do not edit generated output in `dist/`, packaged release output in `release/`, or dependencies in `node_modules/`.

Chrome extension popup documents are short-lived. Keep long-running queue work such as auto-fill and import in `apps/chrome-extension/src/background.ts`, persist progress in `chrome.storage.local`, and have `popup.tsx` trigger it through runtime messages so closing the popup does not cancel the operation.

Production magic-link auth is intentionally strict: normal users get generic, throttled responses, while allowlisted `MAGIC_LINK_DEBUG_EMAILS` such as `tester@example.com` receive a fresh API-only token/link on every request and skip SES email delivery. Preserve that split across web, iOS, and Chrome extension clients.

## Product Implementation Conventions
New vocabulary creation and term updates may create TTS audio work. Audio metadata lives in Postgres (`vocab_audios`, `vocab_audio_jobs`), MP3 bytes live in S3, and the S3 `storage_key` is the source of truth. Do not store MP3 binary data in Postgres or persist public audio URLs as source data. Generate playable URLs dynamically through the API presign path or from `TTS_AUDIO_PUBLIC_BASE_URL` when a public/CDN base is configured. Audio is global across users: identical provider/model/voice/speed/output-format/input-hash combinations should reuse one ready audio record. Keep `speed` in the schema/hash and use `1.00` for the current implementation.

The audio flow is intentionally asynchronous. Creating or updating a vocabulary item should return the card with audio status such as `processing` when a job is queued; the worker generates OpenAI TTS, uploads to S3, then marks audio ready and attaches it. Failures should stay on the job path with capped retries and must not leave inconsistent vocab/audio records. The backfill command should be used for existing production words; lazy play-button generation is not the default behavior.

Bulk import is the default Add experience on both web and iOS. Preserve draft text across refreshes/app exits and append iOS shared words into the bulk import textarea, not a separate queue UI. The iOS Bulk Import page should visually follow the web structure: one main form panel, `Paste your words` label with a compact GPT auto-complete pill, format hint, hidden preview when empty, compact native-swipe preview cards, inline errors, and the import action inside the same panel.

Review-mode UI should stay consistent across web and iOS. Review cards are centered when they fit, but the review screen must allow vertical scrolling when the content exceeds the viewport. The `Next word` / `Show summary` action should remain sticky after an answer is selected. Use icon-only circular audio play buttons beside terms where audio is ready.

Wrong-answer review is session-only and appears on the result page only when the current round has wrong selections. Show a compact centered `Review these again` heading, then cards containing the original word, Chinese, English explanation, and the option the user chose. If there is only one wrong card, render it as a static card rather than a slider. For multiple cards, use the existing horizontal card slider pattern. Visually separate the selected explanation from the selected word/Chinese source, for example with spacing, color, or a small tinted pill.

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
