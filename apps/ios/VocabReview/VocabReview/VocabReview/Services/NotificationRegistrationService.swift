import UIKit

@MainActor
final class NotificationRegistrationService {
    static let shared = NotificationRegistrationService()

    private var continuation: CheckedContinuation<String, Error>?

    private init() {}

    func requestDeviceToken() async throws -> String {
        try await withCheckedThrowingContinuation { continuation in
            self.continuation = continuation
            UIApplication.shared.registerForRemoteNotifications()
        }
    }

    func complete(with deviceToken: Data) {
        guard let continuation else { return }
        self.continuation = nil
        let token = deviceToken.map { String(format: "%02x", $0) }.joined()
        continuation.resume(returning: token)
    }

    func fail(with error: Error) {
        guard let continuation else { return }
        self.continuation = nil
        continuation.resume(throwing: error)
    }
}

final class AppDelegate: NSObject, UIApplicationDelegate {
    func application(_ application: UIApplication, didRegisterForRemoteNotificationsWithDeviceToken deviceToken: Data) {
        Task { @MainActor in
            NotificationRegistrationService.shared.complete(with: deviceToken)
        }
    }

    func application(_ application: UIApplication, didFailToRegisterForRemoteNotificationsWithError error: Error) {
        Task { @MainActor in
            NotificationRegistrationService.shared.fail(with: error)
        }
    }
}
