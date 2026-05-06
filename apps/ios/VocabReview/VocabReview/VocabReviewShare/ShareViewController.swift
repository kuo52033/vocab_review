import UIKit
import UniformTypeIdentifiers

final class ShareViewController: UIViewController {
    private let appGroupID = "group.com.example.VocabReview"
    private let storageKey = "queuedSharedCaptures"

    override func viewDidLoad() {
        super.viewDidLoad()
        view.backgroundColor = UIColor(red: 1.0, green: 0.97, blue: 0.91, alpha: 1.0)
        showSavingMessage()
        collectSharedText()
    }

    private func showSavingMessage() {
        let label = UILabel()
        label.translatesAutoresizingMaskIntoConstraints = false
        label.text = "Saving to Vocab Review..."
        label.textAlignment = .center
        label.font = .preferredFont(forTextStyle: .headline)
        label.textColor = UIColor(red: 0.25, green: 0.33, blue: 0.24, alpha: 1.0)
        view.addSubview(label)
        NSLayoutConstraint.activate([
            label.leadingAnchor.constraint(equalTo: view.leadingAnchor, constant: 20),
            label.trailingAnchor.constraint(equalTo: view.trailingAnchor, constant: -20),
            label.centerYAnchor.constraint(equalTo: view.centerYAnchor)
        ])
    }

    private func collectSharedText() {
        let extensionItems = extensionContext?.inputItems.compactMap { $0 as? NSExtensionItem } ?? []
        let providers = extensionItems.flatMap { $0.attachments ?? [] }
        var capturedText = ""
        var sourceURL = ""
        let group = DispatchGroup()
        let lock = NSLock()

        for provider in providers {
            if provider.hasItemConformingToTypeIdentifier(UTType.plainText.identifier) {
                group.enter()
                provider.loadItem(forTypeIdentifier: UTType.plainText.identifier, options: nil) { item, _ in
                    if let text = item as? String {
                        lock.lock()
                        if capturedText.isEmpty {
                            capturedText = text
                        }
                        lock.unlock()
                    }
                    group.leave()
                }
            }

            if provider.hasItemConformingToTypeIdentifier(UTType.url.identifier) {
                group.enter()
                provider.loadItem(forTypeIdentifier: UTType.url.identifier, options: nil) { item, _ in
                    let urlString: String
                    if let url = item as? URL {
                        urlString = url.absoluteString
                    } else {
                        urlString = item as? String ?? ""
                    }
                    lock.lock()
                    sourceURL = urlString
                    lock.unlock()
                    group.leave()
                }
            }
        }

        group.notify(queue: .main) {
            self.save(text: capturedText, sourceURL: sourceURL)
            self.extensionContext?.completeRequest(returningItems: nil)
        }
    }

    private func save(text: String, sourceURL: String) {
        let term = text.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !term.isEmpty else { return }

        let nextCapture = SharedQueuedCapture(
            id: "\(Date().timeIntervalSince1970)-\(UUID().uuidString)",
            term: term,
            selection: term,
            sourceTitle: "Shared from iOS",
            sourceURL: sourceURL,
            createdAt: ISO8601DateFormatter().string(from: Date())
        )
        var captures = loadQueue()
        captures.insert(nextCapture, at: 0)
        saveQueue(Array(captures.prefix(100)))
    }

    private func loadQueue() -> [SharedQueuedCapture] {
        guard
            let defaults = UserDefaults(suiteName: appGroupID),
            let data = defaults.data(forKey: storageKey),
            let captures = try? JSONDecoder().decode([SharedQueuedCapture].self, from: data)
        else {
            return []
        }
        return captures
    }

    private func saveQueue(_ captures: [SharedQueuedCapture]) {
        guard
            let defaults = UserDefaults(suiteName: appGroupID),
            let data = try? JSONEncoder().encode(captures)
        else {
            return
        }
        defaults.set(data, forKey: storageKey)
    }
}

private struct SharedQueuedCapture: Codable {
    let id: String
    let term: String
    let selection: String
    let sourceTitle: String
    let sourceURL: String
    let createdAt: String
}
