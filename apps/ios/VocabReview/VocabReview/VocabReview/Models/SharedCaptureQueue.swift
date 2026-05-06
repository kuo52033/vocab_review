import Foundation

struct SharedQueuedCapture: Codable, Identifiable {
    let id: String
    let term: String
    let selection: String
    let sourceTitle: String
    let sourceURL: String
    let createdAt: String
}

enum SharedCaptureQueue {
    static let appGroupID = "group.com.example.VocabReview"
    static let storageKey = "queuedSharedCaptures"

    static func load() -> [SharedQueuedCapture] {
        guard
            let defaults = UserDefaults(suiteName: appGroupID),
            let data = defaults.data(forKey: storageKey),
            let captures = try? JSONDecoder().decode([SharedQueuedCapture].self, from: data)
        else {
            return []
        }
        return captures
    }

    static func save(_ captures: [SharedQueuedCapture]) {
        guard
            let defaults = UserDefaults(suiteName: appGroupID),
            let data = try? JSONEncoder().encode(captures)
        else {
            return
        }
        defaults.set(data, forKey: storageKey)
    }

    static func clear() {
        UserDefaults(suiteName: appGroupID)?.removeObject(forKey: storageKey)
    }
}
