import Foundation
import UserNotifications

struct LocalNotificationScheduler {
    private let center: UNUserNotificationCenter

    init(center: UNUserNotificationCenter = .current()) {
        self.center = center
    }

    func scheduleReviewReminders(for cards: [DueCard], now: Date = Date()) async throws -> Int {
        let upcomingCards = cards
            .filter { $0.state.nextDueAtDate > now }
            .sorted { $0.state.nextDueAtDate < $1.state.nextDueAtDate }
            .prefix(20)

        var scheduledCount = 0
        for card in upcomingCards {
            let content = UNMutableNotificationContent()
            content.title = "Time to review vocabulary"
            content.body = "Review \"\(card.item.term)\" now."
            content.sound = .default

            let interval = max(card.state.nextDueAtDate.timeIntervalSince(now), 1)
            let trigger = UNTimeIntervalNotificationTrigger(timeInterval: interval, repeats: false)
            let request = UNNotificationRequest(
                identifier: "vocab-review-\(card.item.id)",
                content: content,
                trigger: trigger
            )
            try await center.add(request)
            scheduledCount += 1
        }
        return scheduledCount
    }
}
