# iOS Notification Token Registration Design

## Goal

Make the existing iOS `Notify` button register an APNs device token with the backend. This prepares the dry-run notification worker to find real device tokens without implementing real APNs delivery yet.

## Scope

This pass only changes the iOS app. The existing backend endpoint `POST /devices/apns-token` remains the persistence path. Registration is explicit: the app does not auto-register after sign-in.

## User Flow

When the signed-in user taps `Notify`:

1. Request notification permission with `UNUserNotificationCenter`.
2. If permission is denied, show `Notification permission was not granted.`
3. If permission is granted, call `UIApplication.shared.registerForRemoteNotifications()`.
4. Wait for the APNs registration callback.
5. Convert the APNs token bytes to a lowercase hex string.
6. Send `{ "platform": "ios", "token": "<hex token>" }` to `/devices/apns-token`.
7. Show `Notification device registered.` when the backend accepts it.

## iOS Architecture

Add an app delegate bridge for APNs callbacks:

- `NotificationRegistrationService`: owns the async request for a remote notification token.
- `AppDelegate`: receives success and failure callbacks from UIKit and forwards them to the service.
- `VocabReviewApp`: wires the app delegate using `@UIApplicationDelegateAdaptor`.
- `SessionStore`: requests permission, asks the registration service for a token, then sends it to the backend.

This keeps UIKit callback handling outside SwiftUI views and keeps the existing `Notify` button as the only trigger.

## Error Handling

If APNs registration fails, show the callback error message. If backend registration fails, reuse existing request error handling. If the user is signed out, the backend request should fail with the existing unauthorized path.

## Testing

Build the iOS app on simulator. Manual testing should tap `Notify`, grant permission, and check whether the backend receives `POST /devices/apns-token`. APNs token callbacks may require a real device depending on simulator/runtime behavior, so a simulator build is the automated verification target for this pass.
