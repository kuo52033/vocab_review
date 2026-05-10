# iOS Local Notifications Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace APNs token registration with local review reminders that work with a Personal Apple development team.

**Architecture:** Keep the existing `Notify` button as the explicit trigger. `SessionStore` requests local notification permission, fetches active library cards, and delegates scheduling to a small `LocalNotificationScheduler`.

**Tech Stack:** SwiftUI, UserNotifications local notifications, existing iOS API client.

---

## Tasks

- [ ] Remove APNs-only app delegate registration and the `aps-environment` entitlement.
- [ ] Add `LocalNotificationScheduler` to create up to 20 one-shot review reminders for future `next_due_at` values.
- [ ] Update `SessionStore.registerNotifications()` to request permission, fetch active cards, schedule local notifications, and show a clear result message.
- [ ] Build the iOS simulator app.

## Self-Review

- Spec coverage: local notifications replace APNs, `Notify` remains explicit, up to 20 reminders are scheduled, and Personal Team signing remains supported.
- Placeholder scan: no TODO/TBD placeholders.
- Type consistency: `LocalNotificationScheduler` accepts `[DueCard]` and uses existing `ReviewState.nextDueAtDate`.
