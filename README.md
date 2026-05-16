# Vocab Review

A monorepo for a personal English vocabulary library with spaced-repetition review, quick capture from Chrome, and notification-first review on iPhone.

## Layout

- `backend/`: Go API server with review scheduling and PostgreSQL persistence
- `apps/web/`: React + TypeScript web app for library management
- `apps/chrome-extension/`: Chrome extension for fast capture
- `apps/ios/`: SwiftUI iPhone app shell for sign-in, review, and notification registration

## Local development

### Postgres

```bash
docker compose up -d postgres
make migrate
```

The local workflow uses:

- `.env.example` for the development connection string template
- `.env.test` for integration-test configuration
- `backend/migrations/` for versioned SQL schema changes

### Backend

```bash
make backend-run
```

### Web

```bash
cd apps/web
npm install
npm run dev
```

### Chrome extension

```bash
cd apps/chrome-extension
npm install
npm run build
```

Load the generated `dist/` directory as an unpacked extension in Chrome.

### iOS

Open `apps/ios/VocabReview` in Xcode and run on an iPhone simulator or device.

## Notes

- The backend now requires PostgreSQL and fails fast if `DATABASE_URL` is missing or the schema has not been migrated.
- Run `make test` for the backend unit suite and `make test-integration` for the Postgres repository integration test path.
- Production deployment lives in `docs/deployment/ec2-caddy-domain.md`, with a repeat-deploy checklist in `docs/deployment/production-checklist.md`.
