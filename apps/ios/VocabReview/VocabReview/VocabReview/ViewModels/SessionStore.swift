import Foundation
import Combine
import UserNotifications

@MainActor
final class SessionStore: ObservableObject {
    @Published var sessionToken: String = UserDefaults.standard.string(forKey: "session_token") ?? ""
    @Published var dueCards: [DueCard] = []
    @Published var libraryCards: [DueCard] = []
    @Published var reviewHistory: [ReviewHistoryEntry] = []
    @Published var reviewStats = ReviewStats(reviewed_today: 0, reviewed_7_days: 0, active_cards: 0, due_now: 0, archived_cards: 0)
    @Published var errorMessage: String = ""
    @Published var infoMessage: String = ""
    @Published var requestedMagicLink: MagicLinkResponse?
    @Published var isRequestingMagicLink: Bool = false
    @Published var isSigningIn: Bool = false
    @Published var isLoadingDueCards: Bool = false
    @Published var isLoadingLibraryCards: Bool = false
    @Published var isLoadingReviewHistory: Bool = false
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
            await refreshAuthenticatedData()
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

    func loadLibraryCards() async {
        guard isAuthenticated else { return }

        isLoadingLibraryCards = true
        errorMessage = ""

        do {
            let response: LibraryResponse = try await sendRequest(path: "/vocab")
            libraryCards = response.items.sorted { lhs, rhs in
                lhs.item.createdAtDate > rhs.item.createdAtDate
            }
        } catch {
            handleRequestError(error)
        }

        isLoadingLibraryCards = false
    }

    func loadReviewHistory() async {
        guard isAuthenticated else { return }

        isLoadingReviewHistory = true
        errorMessage = ""

        do {
            let response: ReviewHistoryResponse = try await sendRequest(path: "/reviews/history")
            reviewHistory = response.items
        } catch {
            handleRequestError(error)
        }

        isLoadingReviewHistory = false
    }

    func loadReviewStats() async {
        guard isAuthenticated else { return }

        do {
            let response: ReviewStatsResponse = try await sendRequest(path: "/reviews/stats")
            reviewStats = response.stats
        } catch {
            handleRequestError(error)
        }
    }

    func refreshAuthenticatedData() async {
        await loadDueCards()
        await loadLibraryCards()
        await loadReviewHistory()
        await loadReviewStats()
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
            await refreshAuthenticatedData()
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
        await createVocab(
            VocabDraftInput(
                term: term,
                meaning: meaning,
                exampleSentence: exampleSentence,
                notes: notes
            )
        )
    }

    func createVocab(_ draft: VocabDraftInput) async -> Bool {
        guard isAuthenticated else { return false }

        let trimmedTerm = draft.term.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !trimmedTerm.isEmpty else {
            errorMessage = "Term is required."
            return false
        }

        isCreatingVocab = true
        errorMessage = ""
        infoMessage = ""

        do {
            let response: CreateVocabResponse = try await sendRequest(
                path: "/vocab",
                method: "POST",
                body: CreateVocabRequest(
                    term: trimmedTerm,
                    kind: "word",
                    meaning: draft.meaning.trimmingCharacters(in: .whitespacesAndNewlines),
                    example_sentence: draft.exampleSentence.trimmingCharacters(in: .whitespacesAndNewlines),
                    source_text: trimmedTerm,
                    source_url: "",
                    notes: draft.notes.trimmingCharacters(in: .whitespacesAndNewlines)
                )
            )
            infoMessage = "Card added."
            applyCreatedVocabCards([response])
            isCreatingVocab = false
            return true
        } catch {
            handleRequestError(error)
            isCreatingVocab = false
            return false
        }
    }

    func createVocabCards(_ drafts: [VocabDraftInput]) async -> Int {
        guard isAuthenticated else { return 0 }

        let trimmedDrafts = drafts
            .map {
                VocabDraftInput(
                    term: $0.term.trimmingCharacters(in: .whitespacesAndNewlines),
                    meaning: $0.meaning.trimmingCharacters(in: .whitespacesAndNewlines),
                    exampleSentence: $0.exampleSentence.trimmingCharacters(in: .whitespacesAndNewlines),
                    notes: $0.notes.trimmingCharacters(in: .whitespacesAndNewlines)
                )
            }
            .filter { !$0.term.isEmpty }

        guard !trimmedDrafts.isEmpty else {
            errorMessage = "Paste at least one term."
            return 0
        }

        isCreatingVocab = true
        errorMessage = ""
        infoMessage = ""
        var createdResponses: [CreateVocabResponse] = []

        for draft in trimmedDrafts {
            do {
                let response: CreateVocabResponse = try await sendRequest(
                    path: "/vocab",
                    method: "POST",
                    body: CreateVocabRequest(
                        term: draft.term,
                        kind: "word",
                        meaning: draft.meaning,
                        example_sentence: draft.exampleSentence,
                        source_text: draft.term,
                        source_url: "",
                        notes: draft.notes
                    )
                )
                createdResponses.append(response)
            } catch {
                handleRequestError(error)
                break
            }
        }

        applyCreatedVocabCards(createdResponses)
        let createdCount = createdResponses.count
        if createdCount > 0 {
            infoMessage = "Imported \(createdCount) \(createdCount == 1 ? "card" : "cards")."
        }
        isCreatingVocab = false
        return createdCount
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
            await refreshAuthenticatedData()
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
            await refreshAuthenticatedData()
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
        libraryCards = []
        reviewHistory = []
        reviewStats = ReviewStats(reviewed_today: 0, reviewed_7_days: 0, active_cards: 0, due_now: 0, archived_cards: 0)
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

    private func applyCreatedVocabCards(_ responses: [CreateVocabResponse]) {
        guard !responses.isEmpty else { return }

        let now = Date()
        let cards = responses.map { DueCard(item: $0.item, state: $0.state) }
        let dueNowCards = cards.filter { $0.state.nextDueAtDate <= now }

        libraryCards.insert(contentsOf: cards, at: 0)
        libraryCards.sort { lhs, rhs in
            lhs.item.createdAtDate > rhs.item.createdAtDate
        }
        dueCards.insert(contentsOf: dueNowCards, at: 0)
        reviewStats = ReviewStats(
            reviewed_today: reviewStats.reviewed_today,
            reviewed_7_days: reviewStats.reviewed_7_days,
            active_cards: reviewStats.active_cards + cards.count,
            due_now: reviewStats.due_now + dueNowCards.count,
            archived_cards: reviewStats.archived_cards
        )
    }
}

extension VocabItem {
    var createdAtDate: Date {
        ISO8601DateFormatter.vocabReview.date(from: created_at) ?? .distantPast
    }
}

extension ReviewState {
    var nextDueAtDate: Date {
        ISO8601DateFormatter.vocabReview.date(from: next_due_at) ?? .distantPast
    }
}

extension ISO8601DateFormatter {
    static let vocabReview: ISO8601DateFormatter = {
        let formatter = ISO8601DateFormatter()
        formatter.formatOptions = [.withInternetDateTime, .withFractionalSeconds]
        return formatter
    }()
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
