# Private Beta Release

This checklist prepares the Chrome extension and iOS app for a one-person private beta against production.

## Decisions

- Distribution: Chrome Web Store unlisted/private testing and TestFlight.
- Authentication: keep the development token flow for this private beta.
- API: production clients point at `https://api.vocabreview.uk`.
- iOS capture: include the share extension.
- iOS notifications: include local reminders only; real APNs push stays out of scope.
- Privacy URL: `https://vocabreview.uk/privacy.html`.
- iOS minimum: iOS 17.0.

## Chrome Extension

Build the production package:

```bash
npm run release:extension
```

Upload `apps/chrome-extension/release/vocab-review-capture.zip` to the Chrome Web Store developer dashboard.
Use unlisted/private testing for the first beta item.

Minimum listing collateral:

- Name: `Vocab Review Capture`
- Short description: `Save words and phrases to your Vocab Review library.`
- Privacy policy: `https://vocabreview.uk/privacy.html`
- Permissions explanation:
  - `storage`: stores sign-in state and queued captures locally.
  - `contextMenus`: adds the right-click capture action.
  - `activeTab`: reads the current selection and page metadata when the popup opens.
  - Host permission for `https://api.vocabreview.uk/*`: sends authenticated captures to the production API.

## iOS TestFlight

Before archiving, confirm these App Store Connect and Apple Developer identifiers exist:

- App bundle ID: `com.tim.VocabReview`
- Share extension bundle ID: `com.tim.VocabReview.VocabReviewShare`
- App group: `group.com.tim.VocabReview`

Archive from Xcode with the `VocabReview` scheme. The scheme runs locally with Debug and archives for production with Release.
The Release configuration reads:

- `VOCAB_REVIEW_API_BASE_URL=https://api.vocabreview.uk`
- `VOCAB_REVIEW_APP_GROUP=group.com.tim.VocabReview`
- `IPHONEOS_DEPLOYMENT_TARGET=17.0`

## Production Smoke Test

Run this after installing the Chrome beta build and iOS TestFlight build:

1. Check `https://api.vocabreview.uk/healthz` returns 200.
2. Open `https://vocabreview.uk/privacy.html`.
3. In Chrome extension, request a magic link, use the returned token, capture selected text, queue multiple right-click captures, auto-complete, and import.
4. In iOS, request a magic link, use the returned token, share text into Vocab Review, import from the shared queue, add a manual card, run a review, and schedule local reminders.
5. In the web app, confirm the same cards and review state are visible.

## Deferred

- Real emailed magic links.
- Public Chrome Web Store listing.
- Public App Store release.
- Real APNs push notifications.
- Self-service account deletion.
