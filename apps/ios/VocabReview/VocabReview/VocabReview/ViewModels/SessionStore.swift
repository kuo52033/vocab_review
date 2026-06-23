import Foundation
import Combine
import UserNotifications
import AVFoundation

@MainActor
final class SessionStore: ObservableObject {
    @Published var sessionToken: String = UserDefaults.standard.string(forKey: "session_token") ?? ""
    @Published var dueCards: [DueCard] = []
    @Published var libraryCards: [DueCard] = []
    @Published var reviewHistory: [ReviewHistoryEntry] = []
    @Published var libraryTotal: Int = 0
    @Published var reviewHistoryTotal: Int = 0
    @Published var libraryPage: Int = 1
    @Published var reviewHistoryPage: Int = 1
    @Published var libraryHasNext: Bool = false
    @Published var reviewHistoryHasNext: Bool = false
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
    @Published var isAutocompletingVocab: Bool = false
    @Published var playingAudioVocabID: String = ""

    private let baseURL = AppEnvironment.apiBaseURL
    private let sessionTokenKey = "session_token"
    private var libraryLoadID = 0
    private var audioPlayer: AVPlayer?
    private var audioEndObserver: NSObjectProtocol?
    let libraryPageSize = 10
    let reviewHistoryPageSize = 21

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
            infoMessage = response.verification_url == nil ? response.message : "Development magic link created."
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
            await loadReviewStats()
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

    func loadLibraryCards(query: String = "", status: String = "") async {
        guard isAuthenticated else { return }

        libraryLoadID += 1
        let loadID = libraryLoadID
        isLoadingLibraryCards = true
        errorMessage = ""
        defer {
            if loadID == libraryLoadID {
                isLoadingLibraryCards = false
            }
        }

        do {
            let response: LibraryResponse = try await sendRequest(path: pathWithQuery("/vocab", [
                URLQueryItem(name: "limit", value: String(libraryPageSize)),
                URLQueryItem(name: "offset", value: String((libraryPage - 1) * libraryPageSize)),
                URLQueryItem(name: "q", value: query.trimmingCharacters(in: .whitespacesAndNewlines)),
                URLQueryItem(name: "status", value: status)
            ]))
            guard loadID == libraryLoadID else { return }
            libraryCards = response.items.sorted { lhs, rhs in
                lhs.item.createdAtDate > rhs.item.createdAtDate
            }
            libraryTotal = response.total ?? response.items.count
            libraryHasNext = response.has_next ?? false
            if libraryCards.isEmpty && libraryPage > 1 && !libraryHasNext {
                libraryPage -= 1
            }
        } catch {
            guard loadID == libraryLoadID else { return }
            handleRequestError(error)
        }
    }

    func loadReviewHistory() async {
        guard isAuthenticated else { return }

        isLoadingReviewHistory = true
        errorMessage = ""

        do {
            let response: ReviewHistoryResponse = try await sendRequest(path: pathWithQuery("/reviews/history", [
                URLQueryItem(name: "limit", value: String(reviewHistoryPageSize)),
                URLQueryItem(name: "offset", value: String((reviewHistoryPage - 1) * reviewHistoryPageSize))
            ]))
            reviewHistory = response.items
            reviewHistoryTotal = response.total ?? response.items.count
            reviewHistoryHasNext = response.has_next ?? false
            if reviewHistory.isEmpty && reviewHistoryPage > 1 && !reviewHistoryHasNext {
                reviewHistoryPage -= 1
            }
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

    func loadReviewSession(limit: Int, candidates: Int) async -> (due: [DueCard], candidates: [ReviewSessionCandidate])? {
        guard isAuthenticated else { return nil }

        isLoadingDueCards = true
        errorMessage = ""
        defer {
            isLoadingDueCards = false
        }

        do {
            let response: ReviewSessionResponse = try await sendRequest(path: pathWithQuery("/reviews/session", [
                URLQueryItem(name: "limit", value: String(limit)),
                URLQueryItem(name: "candidates", value: String(candidates))
            ]))
            dueCards = response.due
            reviewStats = response.stats
            return (response.due, response.candidates)
        } catch {
            handleRequestError(error)
            return nil
        }
    }

    func grade(cardID: String, grade: String) async {
        guard isAuthenticated else { return }

        isGrading = true
        errorMessage = ""
        defer { isGrading = false }

        do {
            let response: ReviewStateResponse = try await sendRequest(
                path: "/reviews/\(cardID)/grade",
                method: "POST",
                body: GradeRequest(grade: grade)
            )
            applyGradedReview(cardID: cardID, state: response.state)
        } catch {
            handleRequestError(error)
        }
    }

    func gradeAndReturnNextDue(cardID: String, grade: String) async -> String? {
        guard isAuthenticated else { return nil }

        isGrading = true
        errorMessage = ""
        defer { isGrading = false }

        do {
            let response: ReviewStateResponse = try await sendRequest(
                path: "/reviews/\(cardID)/grade",
                method: "POST",
                body: GradeRequest(grade: grade)
            )
            applyGradedReview(cardID: cardID, state: response.state)
            return response.state.next_due_at
        } catch {
            handleRequestError(error)
            return nil
        }
    }

    private func applyGradedReview(cardID: String, state: ReviewState) {
        dueCards.removeAll { $0.item.id == cardID }
        libraryCards = libraryCards.map { card in
            card.item.id == cardID ? DueCard(item: card.item, state: state) : card
        }
        reviewStats = ReviewStats(
            reviewed_today: reviewStats.reviewed_today + 1,
            reviewed_7_days: reviewStats.reviewed_7_days + 1,
            active_cards: reviewStats.active_cards,
            due_now: max(0, reviewStats.due_now - 1),
            archived_cards: reviewStats.archived_cards
        )
    }

    func createVocab(
        term: String,
        meaning: String,
        chinese: String,
        exampleSentence: String,
        notes: String
    ) async -> Bool {
        await createVocab(
            VocabDraftInput(
                term: term,
                meaning: meaning,
                chinese: chinese,
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
                    meaning: draft.meaning.trimmingCharacters(in: .whitespacesAndNewlines),
                    chinese: draft.chinese.trimmingCharacters(in: .whitespacesAndNewlines),
                    example_sentence: draft.exampleSentence.trimmingCharacters(in: .whitespacesAndNewlines),
                    part_of_speech: draft.partOfSpeech.trimmingCharacters(in: .whitespacesAndNewlines),
                    source_text: trimmedTerm,
                    source_url: "",
                    notes: draft.notes.trimmingCharacters(in: .whitespacesAndNewlines)
                )
            )
            if response.skipped_duplicate == true {
                infoMessage = "Skipped duplicate \"\(response.item.term)\"."
                isCreatingVocab = false
                return true
            }
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
                    chinese: $0.chinese.trimmingCharacters(in: .whitespacesAndNewlines),
                    exampleSentence: $0.exampleSentence.trimmingCharacters(in: .whitespacesAndNewlines),
                    partOfSpeech: $0.partOfSpeech.trimmingCharacters(in: .whitespacesAndNewlines),
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

        do {
            let response: BulkCreateVocabResponse = try await sendRequest(
                path: "/vocab/bulk",
                method: "POST",
                body: BulkCreateVocabRequest(items: trimmedDrafts.map { draft in
                    CreateVocabRequest(
                        term: draft.term,
                        meaning: draft.meaning,
                        chinese: draft.chinese,
                        example_sentence: draft.exampleSentence,
                        part_of_speech: draft.partOfSpeech,
                        source_text: draft.term,
                        source_url: "",
                        notes: draft.notes
                    )
                })
            )
            let createdResponses = response.items.filter { $0.skipped_duplicate != true }
            applyCreatedVocabCards(createdResponses)
            let createdCount = response.created_count
            let skippedDuplicateCount = response.skipped_duplicate_count
            if createdCount > 0 {
                let skipped = skippedDuplicateCount > 0 ? " Skipped \(skippedDuplicateCount) duplicate\(skippedDuplicateCount == 1 ? "" : "s")." : ""
                infoMessage = "Imported \(createdCount) \(createdCount == 1 ? "card" : "cards").\(skipped)"
            } else if skippedDuplicateCount > 0 {
                infoMessage = "Skipped \(skippedDuplicateCount) duplicate\(skippedDuplicateCount == 1 ? "" : "s")."
            }
            isCreatingVocab = false
            return createdCount
        } catch {
            handleRequestError(error)
            isCreatingVocab = false
            return 0
        }
    }

    func updateVocab(
        cardID: String,
        term: String,
        meaning: String,
        chinese: String,
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
            let response: UpdateVocabResponse = try await sendRequest(
                path: "/vocab/\(cardID)",
                method: "PATCH",
                body: CreateVocabRequest(
                    term: trimmedTerm,
                    meaning: meaning.trimmingCharacters(in: .whitespacesAndNewlines),
                    chinese: chinese.trimmingCharacters(in: .whitespacesAndNewlines),
                    example_sentence: exampleSentence.trimmingCharacters(in: .whitespacesAndNewlines),
                    part_of_speech: "",
                    source_text: trimmedTerm,
                    source_url: "",
                    notes: notes.trimmingCharacters(in: .whitespacesAndNewlines)
                )
            )
            infoMessage = "Card updated."
            applyUpdatedVocab(response.item)
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
            let response: UpdateVocabResponse = try await sendRequest(
                path: "/vocab/\(cardID)",
                method: "DELETE",
                body: EmptyRequest()
            )
            infoMessage = "Card deleted."
            applyArchivedVocab(response.item)
            isDeletingVocab = false
            return true
        } catch {
            handleRequestError(error)
            isDeletingVocab = false
            return false
        }
    }

    func autocompleteVocabCards(_ drafts: [VocabDraftInput]) async -> [VocabDraftInput]? {
        guard isAuthenticated else { return nil }

        let trimmedDrafts = drafts
            .map {
                VocabDraftInput(
                    term: $0.term.trimmingCharacters(in: .whitespacesAndNewlines),
                    meaning: $0.meaning.trimmingCharacters(in: .whitespacesAndNewlines),
                    chinese: $0.chinese.trimmingCharacters(in: .whitespacesAndNewlines),
                    exampleSentence: $0.exampleSentence.trimmingCharacters(in: .whitespacesAndNewlines),
                    partOfSpeech: $0.partOfSpeech.trimmingCharacters(in: .whitespacesAndNewlines),
                    notes: $0.notes.trimmingCharacters(in: .whitespacesAndNewlines)
                )
            }
            .filter { !$0.term.isEmpty }

        guard !trimmedDrafts.isEmpty else {
            errorMessage = "Add at least one term before auto-completing."
            return nil
        }

        isAutocompletingVocab = true
        errorMessage = ""
        infoMessage = ""

        do {
            let response: AutocompleteVocabResponse = try await sendRequest(
                path: "/vocab/autocomplete",
                method: "POST",
                body: AutocompleteVocabRequest(
                    items: trimmedDrafts.map {
                        AutocompleteVocabItem(
                            term: $0.term,
                            meaning: $0.meaning,
                            chinese: $0.chinese,
                            example_sentence: $0.exampleSentence,
                            part_of_speech: $0.partOfSpeech
                        )
                    }
                )
            )
            isAutocompletingVocab = false
            infoMessage = "Auto-completed missing details."
            return mergeAutocompleteSuggestions(cards: trimmedDrafts, suggestions: response.items)
        } catch {
            handleRequestError(error)
            isAutocompletingVocab = false
            return nil
        }
    }

    func registerNotifications() async {
        let center = UNUserNotificationCenter.current()
        errorMessage = ""
        infoMessage = ""
        do {
            let granted = try await center.requestAuthorization(options: [.alert, .sound, .badge])
            if !granted {
                errorMessage = "Notification permission was not granted."
                return
            }
            let cards = try await loadUpcomingNotificationCards()
            let count = try await LocalNotificationScheduler(center: center).scheduleReviewReminders(for: cards)
            if count == 0 {
                infoMessage = "No upcoming reminders to schedule."
            } else {
                infoMessage = "Scheduled \(count) local review reminder\(count == 1 ? "" : "s")."
            }
        } catch {
            handleRequestError(error)
        }
    }

    func toggleAudioPlayback(for item: VocabItem) async {
        guard isAuthenticated else { return }

        if playingAudioVocabID == item.id {
            stopAudioPlayback()
            return
        }

        guard item.hasPlayableAudio else {
            errorMessage = "Pronunciation is still processing."
            return
        }

        stopAudioPlayback()
        errorMessage = ""

        do {
            let urlString: String
            if let directURL = item.audio?.url?.trimmingCharacters(in: .whitespacesAndNewlines), !directURL.isEmpty {
                urlString = directURL
            } else {
                let response: VocabAudioURLResponse = try await sendRequest(path: "/vocab/\(item.id)/audio-url")
                urlString = response.url
            }

            guard let url = URL(string: urlString) else {
                throw SessionStoreError.invalidResponse
            }

            let player = AVPlayer(url: url)
            audioPlayer = player
            playingAudioVocabID = item.id
            audioEndObserver = NotificationCenter.default.addObserver(
                forName: .AVPlayerItemDidPlayToEndTime,
                object: player.currentItem,
                queue: .main
            ) { [weak self] _ in
                Task { @MainActor in
                    self?.stopAudioPlayback()
                }
            }
            player.play()
        } catch {
            stopAudioPlayback()
            errorMessage = "Could not load pronunciation URL for \"\(item.term)\"."
        }
    }

    func stopAudioPlayback() {
        audioPlayer?.pause()
        audioPlayer = nil
        playingAudioVocabID = ""
        if let audioEndObserver {
            NotificationCenter.default.removeObserver(audioEndObserver)
            self.audioEndObserver = nil
        }
    }

    func signOut() {
        stopAudioPlayback()
        sessionToken = ""
        dueCards = []
        libraryCards = []
        reviewHistory = []
        libraryTotal = 0
        reviewHistoryTotal = 0
        libraryPage = 1
        reviewHistoryPage = 1
        libraryHasNext = false
        reviewHistoryHasNext = false
        reviewStats = ReviewStats(reviewed_today: 0, reviewed_7_days: 0, active_cards: 0, due_now: 0, archived_cards: 0)
        requestedMagicLink = nil
        errorMessage = ""
        infoMessage = ""
        isRequestingMagicLink = false
        isSigningIn = false
        isLoadingDueCards = false
        isLoadingLibraryCards = false
        isLoadingReviewHistory = false
        isGrading = false
        isCreatingVocab = false
        isDeletingVocab = false
        isAutocompletingVocab = false
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

    func setLibraryPage(_ page: Int, query: String = "", status: String = "") async {
        libraryPage = max(page, 1)
        await loadLibraryCards(query: query, status: status)
    }

    func setReviewHistoryPage(_ page: Int) async {
        reviewHistoryPage = max(page, 1)
        await loadReviewHistory()
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

    private func loadUpcomingNotificationCards() async throws -> [DueCard] {
        let response: LibraryResponse = try await sendRequest(path: pathWithQuery("/vocab", [
            URLQueryItem(name: "limit", value: "100"),
            URLQueryItem(name: "offset", value: "0")
        ]))
        return response.items
            .sorted { $0.state.nextDueAtDate < $1.state.nextDueAtDate }
            .prefix(20)
            .map { $0 }
    }

    private func rawRequest<Body: Encodable>(
        path: String,
        method: String,
        body: Body?
    ) async throws -> Data {
        guard let url = URL(string: path.hasPrefix("/") ? path : "/\(path)", relativeTo: baseURL)?.absoluteURL else {
            throw SessionStoreError.invalidResponse
        }
        var request = URLRequest(url: url)
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
                signOut()
                errorMessage = "Session expired. Sign in again."
                throw SessionStoreError.unauthorized
            }
            if let apiError = try? JSONDecoder().decode(APIErrorResponse.self, from: data) {
                throw SessionStoreError.message(apiError.error)
            }
            throw SessionStoreError.message("Request failed.")
        }
        return data
    }

    private func pathWithQuery(_ path: String, _ queryItems: [URLQueryItem]) -> String {
        var components = URLComponents()
        components.path = path
        components.queryItems = queryItems.filter { item in
            guard let value = item.value else { return false }
            return !value.isEmpty
        }
        return components.string ?? path
    }

    private func handleRequestError(_ error: Error) {
        if case SessionStoreError.unauthorized = error {
            if isAuthenticated {
                signOut()
            }
            errorMessage = "Session expired. Sign in again."
            return
        }
        errorMessage = error.localizedDescription
    }

    private func mergeAutocompleteSuggestions(cards: [VocabDraftInput], suggestions: [AutocompleteVocabSuggestion]) -> [VocabDraftInput] {
        cards.enumerated().map { index, card in
            guard suggestions.indices.contains(index) else { return card }
            let suggestion = suggestions[index]
            return VocabDraftInput(
                term: card.term,
                meaning: card.meaning.isEmpty ? suggestion.meaning : card.meaning,
                chinese: card.chinese.isEmpty ? suggestion.chinese : card.chinese,
                exampleSentence: card.exampleSentence.isEmpty ? suggestion.example_sentence : card.exampleSentence,
                partOfSpeech: card.partOfSpeech.isEmpty ? normalizedPartOfSpeech(suggestion.part_of_speech) : card.partOfSpeech,
                notes: card.notes
            )
        }
    }

    private func normalizedPartOfSpeech(_ value: String) -> String {
        let normalized = value.trimmingCharacters(in: .whitespacesAndNewlines).lowercased().replacingOccurrences(of: "-", with: "_").replacingOccurrences(of: " ", with: "_")
        let allowed: Set<String> = ["", "noun", "verb", "adjective", "adverb", "phrase", "idiom", "phrasal_verb", "preposition", "conjunction", "interjection", "determiner", "pronoun", "other"]
        if allowed.contains(normalized) {
            return normalized
        }
        return normalized.isEmpty ? "" : "other"
    }

    private func applyCreatedVocabCards(_ responses: [CreateVocabResponse]) {
        guard !responses.isEmpty else { return }

        let now = Date()
        let cards = responses.filter { $0.skipped_duplicate != true }.map { DueCard(item: $0.item, state: $0.state) }
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

    private func applyUpdatedVocab(_ item: VocabItem) {
        libraryCards = libraryCards.map { card in
            card.item.id == item.id ? DueCard(item: item, state: card.state) : card
        }
        dueCards = dueCards.map { card in
            card.item.id == item.id ? DueCard(item: item, state: card.state) : card
        }
    }

    private func applyArchivedVocab(_ item: VocabItem) {
        let wasDue = dueCards.contains { $0.item.id == item.id }
        libraryCards.removeAll { $0.item.id == item.id }
        dueCards.removeAll { $0.item.id == item.id }
        libraryTotal = max(0, libraryTotal - 1)
        reviewStats = ReviewStats(
            reviewed_today: reviewStats.reviewed_today,
            reviewed_7_days: reviewStats.reviewed_7_days,
            active_cards: max(0, reviewStats.active_cards - 1),
            due_now: max(0, reviewStats.due_now - (wasDue ? 1 : 0)),
            archived_cards: reviewStats.archived_cards + 1
        )
    }
}

extension VocabItem {
    var createdAtDate: Date {
        ISO8601DateFormatter.vocabReview.date(from: created_at) ?? .distantPast
    }

    var hasPlayableAudio: Bool {
        guard audio?.status == "ready" else { return false }
        let hasDirectURL = !(audio?.url?.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty ?? true)
        let hasStorageKey = !(audio?.storage_key?.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty ?? true)
        return hasDirectURL || hasStorageKey
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
