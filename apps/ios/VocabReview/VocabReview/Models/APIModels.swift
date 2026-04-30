import Foundation

struct AuthResponse: Codable {
    let session: SessionPayload
}

struct SessionPayload: Codable {
    let token: String
}

struct DueResponse: Codable {
    let items: [DueCard]
}

struct DueCard: Codable, Identifiable {
    let item: VocabItem
    let state: ReviewState

    var id: String { item.id }
}

struct VocabItem: Codable {
    let id: String
    let term: String
    let kind: String
    let meaning: String
    let example_sentence: String
    let source_text: String
    let source_url: String
    let notes: String
}

struct ReviewState: Codable {
    let vocab_item_id: String
    let status: String
    let ease_factor: Double
    let interval_days: Int
    let repetition_count: Int
    let next_due_at: String
}
