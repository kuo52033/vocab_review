# Vocab Review

A monorepo for a personal English vocabulary library with spaced-repetition review, quick capture from Chrome, and notification-first review on iPhone.

## Layout

- `backend/`: Go API server with review scheduling and PostgreSQL persistence
- `apps/web/`: React + TypeScript web app for library management
- `apps/chrome-extension/`: Chrome extension for fast capture, queue enrichment, and background import
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
For a private beta production package, run this from the repository root:

```bash
npm run release:extension
```

The release command builds against `https://api.vocabreview.uk` and packages `apps/chrome-extension/release/vocab-review-capture.zip`. The zip should contain only `manifest.json`, `popup.html`, and files under `assets/`.

Extension queue work is background-owned. `Fill missing` and `Import cards` run in the Chrome background service worker and persist progress through `chrome.storage.local`, so closing and reopening the popup should not require repeating the same action or re-importing the same queued cards.

### iOS

Open `apps/ios/VocabReview` in Xcode and run on an iPhone simulator or device.

Private beta release notes for Chrome Web Store and TestFlight live in `docs/private-beta-release.md`.

## Notes

- The backend now requires PostgreSQL and fails fast if `DATABASE_URL` is missing or the schema has not been migrated.
- Production magic-link sign-in returns generic throttled responses for normal users. Allowlisted debug emails such as `tester@example.com` return a fresh token and verification URL on every request for tester flows, without sending SES email.
- Run `make test` for the backend unit suite and `make test-integration` for the Postgres repository integration test path.
- Production deployment automation is documented in `docs/deployment.md`.
