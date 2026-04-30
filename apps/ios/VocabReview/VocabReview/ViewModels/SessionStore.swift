import Foundation
import UserNotifications

@MainActor
final class SessionStore: ObservableObject {
    @Published var sessionToken: String = UserDefaults.standard.string(forKey: "session_token") ?? ""
    @Published var dueCards: [DueCard] = []
    @Published var errorMessage: String = ""

    private let baseURL = URL(string: "http://localhost:8080")!

    func signIn(with magicToken: String) async {
        do {
            var request = URLRequest(url: baseURL.appendingPathComponent("/auth/verify"))
            request.httpMethod = "POST"
            request.setValue("application/json", forHTTPHeaderField: "Content-Type")
            request.httpBody = try JSONEncoder().encode(["token": magicToken])
            let (data, _) = try await URLSession.shared.data(for: request)
            let response = try JSONDecoder().decode(AuthResponse.self, from: data)
            sessionToken = response.session.token
            UserDefaults.standard.set(response.session.token, forKey: "session_token")
            await loadDueCards()
        } catch {
            errorMessage = error.localizedDescription
        }
    }

    func loadDueCards() async {
        guard !sessionToken.isEmpty else { return }
        do {
            var request = URLRequest(url: baseURL.appendingPathComponent("/reviews/due"))
            request.setValue("Bearer \(sessionToken)", forHTTPHeaderField: "Authorization")
            let (data, _) = try await URLSession.shared.data(for: request)
            let response = try JSONDecoder().decode(DueResponse.self, from: data)
            dueCards = response.items
        } catch {
            errorMessage = error.localizedDescription
        }
    }

    func grade(cardID: String, grade: String) async {
        guard !sessionToken.isEmpty else { return }
        do {
            var request = URLRequest(url: baseURL.appendingPathComponent("/reviews/\(cardID)/grade"))
            request.httpMethod = "POST"
            request.setValue("Bearer \(sessionToken)", forHTTPHeaderField: "Authorization")
            request.setValue("application/json", forHTTPHeaderField: "Content-Type")
            request.httpBody = try JSONEncoder().encode(["grade": grade])
            _ = try await URLSession.shared.data(for: request)
            await loadDueCards()
        } catch {
            errorMessage = error.localizedDescription
        }
    }

    func registerNotifications() async {
        let center = UNUserNotificationCenter.current()
        do {
            let granted = try await center.requestAuthorization(options: [.alert, .sound, .badge])
            if !granted {
                errorMessage = "Notification permission was not granted."
            }
        } catch {
            errorMessage = error.localizedDescription
        }
    }
}
