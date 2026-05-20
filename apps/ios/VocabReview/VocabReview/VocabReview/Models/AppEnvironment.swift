import Foundation

enum AppEnvironment {
    static let apiBaseURL: URL = {
        let value = Bundle.main.object(forInfoDictionaryKey: "VocabReviewAPIBaseURL") as? String ?? "http://localhost:8080"
        guard let url = URL(string: value) else {
            preconditionFailure("Invalid VocabReviewAPIBaseURL: \(value)")
        }
        return url
    }()
}
