# Vocab Review

A monorepo for a personal English vocabulary library with spaced-repetition review, quick capture from Chrome, and notification-first review on iPhone.

## Layout

- `backend/`: Go API server with review scheduling and local JSON persistence
- `apps/web/`: React + TypeScript web app for library management
- `apps/chrome-extension/`: Chrome extension for fast capture
- `apps/ios/`: SwiftUI iPhone app shell for sign-in, review, and notification registration

## Local development

### Backend

```bash
cd backend
go run ./cmd/api
```

The backend stores development data in `backend/data/dev-store.json`.

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

- The current backend uses a JSON-backed repository to keep the project runnable without external services.
- The API and service boundaries are structured so PostgreSQL, APNs, and email delivery can be added without rewriting the clients.
