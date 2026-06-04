import UIKit
import UniformTypeIdentifiers

final class ShareViewController: UIViewController {
    private let appGroupID = Bundle.main.object(forInfoDictionaryKey: "VocabReviewAppGroup") as? String ?? "group.com.tim.VocabReview"
    private let storageKey = "queuedSharedCaptures"
    private var hasAppeared = false
    private var pendingCompletion = false
    private var didRequestCompletion = false

    override func viewDidLoad() {
        super.viewDidLoad()
        view.backgroundColor = UIColor(red: 1.0, green: 0.97, blue: 0.91, alpha: 1.0)
        showSavingMessage()
        collectSharedText()
    }

    override func viewDidAppear(_ animated: Bool) {
        super.viewDidAppear(animated)
        hasAppeared = true
        if pendingCompletion {
            requestExtensionCompletion()
        }
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
        if let itemText = sharedText(from: extensionItems) {
            saveAndComplete(text: itemText, sourceURL: "")
            return
        }

        let providers = extensionItems.flatMap { $0.attachments ?? [] }
        let textTypeIdentifiers = [
            UTType.plainText.identifier,
            UTType.text.identifier,
            "public.utf8-plain-text"
        ]

        for provider in providers {
            if let typeIdentifier = textTypeIdentifiers.first(where: { provider.hasItemConformingToTypeIdentifier($0) }) {
                loadTextData(from: provider, typeIdentifier: typeIdentifier)
                return
            }
        }

        saveAndComplete(text: "", sourceURL: "")
    }

    private func saveAndComplete(text: String, sourceURL: String) {
        save(text: text, sourceURL: sourceURL)
        requestExtensionCompletion()
    }

    private func requestExtensionCompletion() {
        guard hasAppeared else {
            pendingCompletion = true
            return
        }
        guard !didRequestCompletion else { return }

        pendingCompletion = false
        didRequestCompletion = true
        DispatchQueue.main.async { [weak self] in
            guard let self else { return }
            self.extensionContext?.completeRequest(returningItems: nil, completionHandler: nil)
            DispatchQueue.main.asyncAfter(deadline: .now() + 1.0) { [weak self] in
                guard let self, self.view.window != nil else { return }
                self.dismiss(animated: false)
            }
        }
    }

    private func loadTextData(from provider: NSItemProvider, typeIdentifier: String) {
        var didComplete = false

        let completeOnce: (String) -> Void = { [weak self] text in
            DispatchQueue.main.async {
                guard let self, !didComplete else { return }
                didComplete = true
                self.saveAndComplete(text: text, sourceURL: "")
            }
        }

        provider.loadDataRepresentation(forTypeIdentifier: typeIdentifier) { [weak self] data, _ in
            let text = data
                .flatMap { String(data: $0, encoding: .utf8) }
                .map { self?.cleanedSharedText($0) ?? "" } ?? ""
            completeOnce(text)
        }

        DispatchQueue.main.asyncAfter(deadline: .now() + 1.5) {
            completeOnce("")
        }
    }

    private func sharedText(from extensionItems: [NSExtensionItem]) -> String? {
        for item in extensionItems {
            if let text = item.attributedContentText?.string, !cleanedSharedText(text).isEmpty {
                return cleanedSharedText(text)
            }
            if let text = item.attributedTitle?.string, !cleanedSharedText(text).isEmpty {
                return cleanedSharedText(text)
            }
        }
        return nil
    }

    private func cleanedSharedText(_ text: String) -> String {
        text.trimmingCharacters(in: .whitespacesAndNewlines)
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
