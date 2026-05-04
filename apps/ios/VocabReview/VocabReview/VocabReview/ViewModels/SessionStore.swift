import Foundation
import Combine
import UserNotifications

@MainActor
final class SessionStore: ObservableObject {
    @Published var sessionToken: String = UserDefaults.standard.string(forKey: "session_token") ?? ""
    @Published var dueCards: [DueCard] = []
    @Published var errorMessage: String = ""
    @Published var infoMessage: String = ""
    @Published var requestedMagicLink: MagicLinkResponse?
    @Published var isRequestingMagicLink: Bool = false
    @Published var isSigningIn: Bool = false
    @Published var isLoadingDueCards: Bool = false
    @Published var isGrading: Bool = false
    @Published var isCreatingVocab: Bool = false
    @Published var isDeletingVocab: Bool = false

    private let baseURL = URL(string: "http://localhost:8080")!
    private let sessionTokenKey = "session_token"

    var isAuthenticated: Bool {
        !sessionToken.isEmpty
    }

    var currentCard: DueCard? {
        dueCards.first
    }

    func requestMagicLink(for email: String) async {
        let trimmedEmail = email.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !trimmedEmail.isEmpty else {
            errorMessage = "Email is required."
            return
        }

        isRequestingMagicLink = true
        errorMessage = ""
        infoMessage = ""

        do {
            let response: MagicLinkResponse = try await sendRequest(
                path: "/auth/magic-link",
                method: "POST",
                body: MagicLinkRequest(email: trimmedEmail, base_url: baseURL.absoluteString)
            )
            requestedMagicLink = response
            infoMessage = "Development magic link created."
        } catch {
            errorMessage = error.localizedDescription
        }

        isRequestingMagicLink = false
    }

    func signIn(with magicToken: String) async {
        let trimmedToken = magicToken.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !trimmedToken.isEmpty else {
            errorMessage = "Magic token is required."
            return
        }

        isSigningIn = true
        errorMessage = ""
        infoMessage = ""

        do {
            let response: AuthResponse = try await sendRequest(
                path: "/auth/verify",
                method: "POST",
                body: VerifyRequest(token: trimmedToken)
            )
            sessionToken = response.session.token
            UserDefaults.standard.set(response.session.token, forKey: sessionTokenKey)
            requestedMagicLink = nil
            await loadDueCards()
        } catch {
            errorMessage = error.localizedDescription
        }

        isSigningIn = false
    }

    func loadDueCards() async {
        guard isAuthenticated else { return }

        isLoadingDueCards = true
        errorMessage = ""

        do {
            let response: DueResponse = try await sendRequest(path: "/reviews/due")
            dueCards = response.items
        } catch {
            handleRequestError(error)
        }

        isLoadingDueCards = false
    }

    func grade(cardID: String, grade: String) async {
        guard isAuthenticated else { return }

        isGrading = true
        errorMessage = ""

        do {
            let _: ReviewStateResponse = try await sendRequest(
                path: "/reviews/\(cardID)/grade",
                method: "POST",
                body: GradeRequest(grade: grade)
            )
            await loadDueCards()
        } catch {
            handleRequestError(error)
        }

        isGrading = false
    }

    func createVocab(
        term: String,
        meaning: String,
        exampleSentence: String,
        notes: String
    ) async -> Bool {
        guard isAuthenticated else { return false }

        let trimmedTerm = term.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !trimmedTerm.isEmpty else {
            errorMessage = "Term is required."
            return false
        }

        isCreatingVocab = true
        errorMessage = ""
        infoMessage = ""

        do {
            let _: CreateVocabResponse = try await sendRequest(
                path: "/vocab",
                method: "POST",
                body: CreateVocabRequest(
                    term: trimmedTerm,
                    kind: "word",
                    meaning: meaning.trimmingCharacters(in: .whitespacesAndNewlines),
                    example_sentence: exampleSentence.trimmingCharacters(in: .whitespacesAndNewlines),
                    source_text: trimmedTerm,
                    source_url: "",
                    notes: notes.trimmingCharacters(in: .whitespacesAndNewlines)
                )
            )
            infoMessage = "Card added."
            await loadDueCards()
            isCreatingVocab = false
            return true
        } catch {
            handleRequestError(error)
            isCreatingVocab = false
            return false
        }
    }

    func updateVocab(
        cardID: String,
        term: String,
        meaning: String,
        exampleSentence: String,
        notes: String
    ) async -> Bool {
        guard isAuthenticated else { return false }

        let trimmedTerm = term.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !trimmedTerm.isEmpty else {
            errorMessage = "Term is required."
            return false
        }

        isCreatingVocab = true
        errorMessage = ""
        infoMessage = ""

        do {
            let _: UpdateVocabResponse = try await sendRequest(
                path: "/vocab/\(cardID)",
                method: "PATCH",
                body: CreateVocabRequest(
                    term: trimmedTerm,
                    kind: "word",
                    meaning: meaning.trimmingCharacters(in: .whitespacesAndNewlines),
                    example_sentence: exampleSentence.trimmingCharacters(in: .whitespacesAndNewlines),
                    source_text: trimmedTerm,
                    source_url: "",
                    notes: notes.trimmingCharacters(in: .whitespacesAndNewlines)
                )
            )
            infoMessage = "Card updated."
            await loadDueCards()
            isCreatingVocab = false
            return true
        } catch {
            handleRequestError(error)
            isCreatingVocab = false
            return false
        }
    }

    func deleteVocab(cardID: String) async -> Bool {
        guard isAuthenticated else { return false }

        isDeletingVocab = true
        errorMessage = ""
        infoMessage = ""

        do {
            let _: UpdateVocabResponse = try await sendRequest(
                path: "/vocab/\(cardID)",
                method: "DELETE",
                body: EmptyRequest()
            )
            infoMessage = "Card deleted."
            await loadDueCards()
            isDeletingVocab = false
            return true
        } catch {
            handleRequestError(error)
            isDeletingVocab = false
            return false
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

    func signOut() {
        sessionToken = ""
        dueCards = []
        requestedMagicLink = nil
        errorMessage = ""
        infoMessage = ""
        isCreatingVocab = false
        isDeletingVocab = false
        UserDefaults.standard.removeObject(forKey: sessionTokenKey)
    }

    func useRequestedMagicLink() async {
        guard let token = requestedMagicLink?.token ?? requestedMagicLinkToken() else {
            errorMessage = "No development magic link is available yet."
            return
        }
        await signIn(with: token)
    }

    func clearError() {
        errorMessage = ""
    }

    private func requestedMagicLinkToken() -> String? {
        guard
            let urlString = requestedMagicLink?.verification_url,
            let components = URLComponents(string: urlString)
        else {
            return nil
        }
        return components.queryItems?.first(where: { $0.name == "token" })?.value
    }

    private func sendRequest<T: Decodable, Body: Encodable>(
        path: String,
        method: String = "GET",
        body: Body? = nil
    ) async throws -> T {
        let data = try await rawRequest(path: path, method: method, body: body)
        return try JSONDecoder().decode(T.self, from: data)
    }

    private func sendRequest<T: Decodable>(
        path: String,
        method: String = "GET"
    ) async throws -> T {
        let data = try await rawRequest(path: path, method: method, body: Optional<String>.none)
        return try JSONDecoder().decode(T.self, from: data)
    }

    private func rawRequest<Body: Encodable>(
        path: String,
        method: String,
        body: Body?
    ) async throws -> Data {
        var request = URLRequest(url: baseURL.appendingPathComponent(path.trimmingCharacters(in: CharacterSet(charactersIn: "/"))))
        request.httpMethod = method
        request.setValue("application/json", forHTTPHeaderField: "Content-Type")
        if isAuthenticated {
            request.setValue("Bearer \(sessionToken)", forHTTPHeaderField: "Authorization")
        }
        if let body {
            request.httpBody = try JSONEncoder().encode(body)
        }

        let (data, response) = try await URLSession.shared.data(for: request)
        guard let httpResponse = response as? HTTPURLResponse else {
            throw SessionStoreError.invalidResponse
        }
        guard (200..<300).contains(httpResponse.statusCode) else {
            if httpResponse.statusCode == 401 {
                throw SessionStoreError.unauthorized
            }
            if let apiError = try? JSONDecoder().decode(APIErrorResponse.self, from: data) {
                throw SessionStoreError.message(apiError.error)
            }
            throw SessionStoreError.message("Request failed.")
        }
        return data
    }

    private func handleRequestError(_ error: Error) {
        if case SessionStoreError.unauthorized = error {
            signOut()
            errorMessage = "Session expired. Sign in again."
            return
        }
        errorMessage = error.localizedDescription
    }
}

struct ReviewStateResponse: Codable {
    let state: ReviewState
}

enum SessionStoreError: LocalizedError {
    case invalidResponse
    case unauthorized
    case message(String)

    var errorDescription: String? {
        switch self {
        case .invalidResponse:
            return "The server returned an invalid response."
        case .unauthorized:
            return "Session expired. Sign in again."
        case let .message(message):
            return message
        }
    }
}
