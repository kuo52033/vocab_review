import Foundation

struct MagicLinkResponse: Codable {
    let token: String
    let verification_url: String
    let expires_at: String
}

struct AuthResponse: Codable {
    let session: SessionPayload
}

struct SessionPayload: Codable {
    let token: String
}

struct DueResponse: Codable {
    let items: [DueCard]
}

struct LibraryResponse: Codable {
    let items: [DueCard]
    let total: Int?
    let limit: Int?
    let offset: Int?
}

struct ReviewHistoryResponse: Codable {
    let items: [ReviewHistoryEntry]
    let total: Int?
    let limit: Int?
    let offset: Int?
}

struct ReviewStatsResponse: Codable {
    let stats: ReviewStats
}

struct ReviewStats: Codable {
    let reviewed_today: Int
    let reviewed_7_days: Int
    let active_cards: Int
    let due_now: Int
    let archived_cards: Int
}

struct CreateVocabResponse: Codable {
    let item: VocabItem
    let state: ReviewState
}

struct UpdateVocabResponse: Codable {
    let item: VocabItem
}

struct DueCard: Codable, Identifiable {
    let item: VocabItem
    let state: ReviewState

    var id: String { item.id }
}

struct ReviewHistoryEntry: Codable, Identifiable {
    let log: ReviewLog
    let item: VocabItem
    let state: ReviewState

    var id: String { log.id }
}

struct ReviewLog: Codable {
    let id: String
    let user_id: String
    let vocab_item_id: String
    let grade: String
    let reviewed_at: String
}

struct VocabItem: Codable {
    let id: String
    let user_id: String
    let term: String
    let meaning: String
    let example_sentence: String
    let part_of_speech: String
    let source_text: String
    let source_url: String
    let notes: String
    let created_at: String
    let updated_at: String
    let archived_at: String?
}

struct ReviewState: Codable {
    let vocab_item_id: String
    let status: String
    let ease_factor: Double
    let interval_days: Int
    let repetition_count: Int
    let next_due_at: String
}

struct GradeRequest: Codable {
    let grade: String
}

struct MagicLinkRequest: Codable {
    let email: String
    let base_url: String
}

struct VerifyRequest: Codable {
    let token: String
}

struct CreateVocabRequest: Codable {
    let term: String
    let meaning: String
    let example_sentence: String
    let part_of_speech: String
    let source_text: String
    let source_url: String
    let notes: String
}

struct VocabDraftInput {
    let term: String
    let meaning: String
    let exampleSentence: String
    let partOfSpeech: String
    let notes: String

    init(term: String, meaning: String, exampleSentence: String, partOfSpeech: String = "", notes: String) {
        self.term = term
        self.meaning = meaning
        self.exampleSentence = exampleSentence
        self.partOfSpeech = partOfSpeech
        self.notes = notes
    }
}

struct EmptyRequest: Encodable {}

struct APIErrorResponse: Codable {
    let error: String
}

struct AutocompleteVocabRequest: Codable {
    let items: [AutocompleteVocabItem]
}

struct AutocompleteVocabItem: Codable {
    let term: String
    let meaning: String
    let example_sentence: String
    let part_of_speech: String
}

struct AutocompleteVocabResponse: Codable {
    let items: [AutocompleteVocabSuggestion]
}

struct AutocompleteVocabSuggestion: Codable {
    let term: String
    let meaning: String
    let example_sentence: String
    let part_of_speech: String
    let error: String
}
