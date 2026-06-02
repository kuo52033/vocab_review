import Foundation

struct MagicLinkResponse: Codable {
    let message: String
    let token: String?
    let verification_url: String?
    let expires_at: String?
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
    let created: Bool?
    let skipped_duplicate: Bool?
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
    let chinese: String
    let example_sentence: String
    let part_of_speech: String
    let source_text: String
    let source_url: String
    let notes: String
    let audio: VocabAudio?
    let created_at: String
    let updated_at: String
    let archived_at: String?

    enum CodingKeys: String, CodingKey {
        case id
        case user_id
        case term
        case meaning
        case chinese
        case example_sentence
        case part_of_speech
        case source_text
        case source_url
        case notes
        case audio
        case created_at
        case updated_at
        case archived_at
    }

    init(from decoder: Decoder) throws {
        let container = try decoder.container(keyedBy: CodingKeys.self)
        id = try container.decode(String.self, forKey: .id)
        user_id = try container.decode(String.self, forKey: .user_id)
        term = try container.decode(String.self, forKey: .term)
        meaning = try container.decode(String.self, forKey: .meaning)
        chinese = try container.decodeIfPresent(String.self, forKey: .chinese) ?? ""
        example_sentence = try container.decode(String.self, forKey: .example_sentence)
        part_of_speech = try container.decode(String.self, forKey: .part_of_speech)
        source_text = try container.decode(String.self, forKey: .source_text)
        source_url = try container.decode(String.self, forKey: .source_url)
        notes = try container.decode(String.self, forKey: .notes)
        audio = try container.decodeIfPresent(VocabAudio.self, forKey: .audio)
        created_at = try container.decode(String.self, forKey: .created_at)
        updated_at = try container.decode(String.self, forKey: .updated_at)
        archived_at = try container.decodeIfPresent(String.self, forKey: .archived_at)
    }
}

struct VocabAudio: Codable {
    let status: String
    let storage_key: String?
    let url: String?
    let provider: String?
    let model: String?
    let voice: String?
    let speed: Double?
    let output_format: String?
}

struct VocabAudioURLResponse: Codable {
    let url: String
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
    let chinese: String
    let example_sentence: String
    let part_of_speech: String
    let source_text: String
    let source_url: String
    let notes: String
}

struct VocabDraftInput {
    let term: String
    let meaning: String
    let chinese: String
    let exampleSentence: String
    let partOfSpeech: String
    let notes: String

    init(term: String, meaning: String, chinese: String = "", exampleSentence: String, partOfSpeech: String = "", notes: String) {
        self.term = term
        self.meaning = meaning
        self.chinese = chinese
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
    let chinese: String
    let example_sentence: String
    let part_of_speech: String
}

struct AutocompleteVocabResponse: Codable {
    let items: [AutocompleteVocabSuggestion]
}

struct AutocompleteVocabSuggestion: Codable {
    let term: String
    let meaning: String
    let chinese: String
    let example_sentence: String
    let part_of_speech: String
    let error: String

    enum CodingKeys: String, CodingKey {
        case term
        case meaning
        case chinese
        case example_sentence
        case part_of_speech
        case error
    }

    init(from decoder: Decoder) throws {
        let container = try decoder.container(keyedBy: CodingKeys.self)
        term = try container.decode(String.self, forKey: .term)
        meaning = try container.decode(String.self, forKey: .meaning)
        chinese = try container.decodeIfPresent(String.self, forKey: .chinese) ?? ""
        example_sentence = try container.decode(String.self, forKey: .example_sentence)
        part_of_speech = try container.decode(String.self, forKey: .part_of_speech)
        error = try container.decode(String.self, forKey: .error)
    }
}
