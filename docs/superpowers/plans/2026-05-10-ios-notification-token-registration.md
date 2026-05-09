# iOS Notification Token Registration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the iOS `Notify` button register an APNs device token with the existing backend endpoint.

**Architecture:** Add a UIKit app delegate bridge that converts APNs callbacks into an async token request. Keep `SessionStore` as the orchestration point that requests permission, awaits a token, and posts it to `/devices/apns-token`.

**Tech Stack:** SwiftUI, UIKit remote notification registration callbacks, UserNotifications, existing iOS API client.

---

## File Structure

- Create `apps/ios/VocabReview/VocabReview/VocabReview/Services/NotificationRegistrationService.swift`: async APNs registration service and app delegate.
- Modify `apps/ios/VocabReview/VocabReview/VocabReview/VocabReviewApp.swift`: wire `@UIApplicationDelegateAdaptor`.
- Modify `apps/ios/VocabReview/VocabReview/VocabReview/Models/APIModels.swift`: add request/response DTOs.
- Modify `apps/ios/VocabReview/VocabReview/VocabReview/ViewModels/SessionStore.swift`: request permission, await APNs token, register token with backend.

## Task 1: App Delegate Token Bridge

- [ ] Create `NotificationRegistrationService` with `requestDeviceToken() async throws -> String`.
- [ ] Add `AppDelegate` methods `didRegisterForRemoteNotificationsWithDeviceToken` and `didFailToRegisterForRemoteNotificationsWithError`.
- [ ] Convert token bytes to lowercase hex with `map { String(format: "%02x", $0) }.joined()`.

## Task 2: SessionStore Backend Registration

- [ ] Add `DeviceTokenRequest` and `DeviceTokenResponse` DTOs.
- [ ] Update `registerNotifications()` to request permission, call `NotificationRegistrationService.shared.requestDeviceToken()`, then `POST /devices/apns-token`.
- [ ] Set success message to `Notification device registered.`

## Task 3: Verify

- [ ] Build the iOS simulator app with XcodeBuildMCP.
- [ ] Commit the implementation and plan when the build passes.

## Self-Review

- Spec coverage: explicit Notify button flow, permission denied behavior, APNs token conversion, backend registration, and simulator build verification are covered.
- Placeholder scan: no TODO/TBD placeholders.
- Type consistency: `DeviceTokenRequest`, `DeviceTokenResponse`, and `NotificationRegistrationService.shared` are used consistently.
