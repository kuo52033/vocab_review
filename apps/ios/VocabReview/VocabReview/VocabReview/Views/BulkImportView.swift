import SwiftUI

struct BulkImportView: View {
    @EnvironmentObject private var sessionStore: SessionStore
    @Environment(\.dismiss) private var dismiss
    @Environment(\.scenePhase) private var scenePhase
    private let presentation: AddCardsPresentation
    @State private var rawText = BulkImportDraftStorage.load()
    @State private var parsedCards: [VocabDraftInput] = []
    @State private var enrichedCards: [VocabDraftInput]?
    @FocusState private var isImportTextFocused: Bool

    private var importCandidates: [VocabDraftInput] {
        enrichedCards ?? parsedCards
    }

    init(presentation: AddCardsPresentation = .standalone) {
        self.presentation = presentation
    }

    var body: some View {
        switch presentation {
        case .standalone:
            NavigationStack {
                ZStack {
                    ReadingDeskBackground()

                    ScrollView {
                        importContent(showHeader: true)
                            .padding()
                    }
                }
                .toolbar(.hidden, for: .navigationBar)
                .dismissKeyboardOnTapOutside()
            }
            .task { refreshDraftAndSharedCaptures() }
            .onAppear { refreshDraftAndSharedCaptures() }
            .onChange(of: scenePhase) { _, newPhase in
                if newPhase == .active {
                    refreshDraftAndSharedCaptures()
                }
            }
            .onChange(of: rawText) { _, newValue in syncParsedCards(with: newValue) }
        case .embedded:
            importContent(showHeader: false)
                .task { refreshDraftAndSharedCaptures() }
                .onAppear { refreshDraftAndSharedCaptures() }
                .onChange(of: scenePhase) { _, newPhase in
                    if newPhase == .active {
                        refreshDraftAndSharedCaptures()
                    }
                }
                .onChange(of: rawText) { _, newValue in syncParsedCards(with: newValue) }
        }
    }

    private func importContent(showHeader: Bool) -> some View {
        VStack(alignment: .leading, spacing: 18) {
            if showHeader {
                VStack(alignment: .leading, spacing: 8) {
                    Text("Bulk import")
                        .readingTitle()
                    Text("Shared words appear in the text box automatically, or paste one card per line.")
                        .readingMuted()
                }
            }

            VStack(alignment: .leading, spacing: 14) {
                HStack(alignment: .center, spacing: 12) {
                    Text("Paste your words")
                        .font(AppTheme.uiFont(size: 15, weight: .bold, relativeTo: .subheadline))
                        .foregroundStyle(AppTheme.ink)

                    Spacer(minLength: 8)

                    Button {
                        Task { await autocompleteCards() }
                    } label: {
                        HStack(spacing: 7) {
                            if sessionStore.isAutocompletingVocab {
                                ProgressView()
                                    .controlSize(.mini)
                            } else {
                                Circle()
                                    .fill(AppTheme.paper)
                                    .frame(width: 8, height: 8)
                            }

                            Text(sessionStore.isAutocompletingVocab ? "Working..." : "GPT Auto-complete")
                                .lineLimit(1)
                        }
                        .font(AppTheme.uiFont(size: 12, weight: .bold, relativeTo: .caption))
                        .foregroundStyle(AppTheme.paper)
                        .padding(.horizontal, 11)
                        .padding(.vertical, 8)
                        .background(
                            AppTheme.coral.opacity(autocompleteDisabled ? 0.42 : 1),
                            in: Capsule()
                        )
                        .overlay {
                            Capsule()
                                .stroke(AppTheme.lineStrong.opacity(autocompleteDisabled ? 0.24 : 0.7), lineWidth: 1)
                        }
                    }
                    .buttonStyle(.plain)
                    .disabled(autocompleteDisabled)
                }

                TextEditor(text: $rawText)
                    .frame(minHeight: 190)
                    .scrollContentBackground(.hidden)
                    .foregroundStyle(AppTheme.ink)
                    .tint(AppTheme.coral)
                    .focused($isImportTextFocused)
                    .padding(10)
                    .background(AppTheme.paper.opacity(0.82), in: RoundedRectangle(cornerRadius: 16, style: .continuous))
                    .overlay {
                        RoundedRectangle(cornerRadius: 16, style: .continuous)
                            .stroke(AppTheme.ink.opacity(0.08), lineWidth: 1)
                    }

                Text("One card per line. Full format: word | definition | 中文 | example sentence | part_of_speech")
                    .font(.footnote)
                    .readingMuted()

                previewSection

                if !sessionStore.errorMessage.isEmpty {
                    Text(sessionStore.errorMessage)
                        .font(.footnote.weight(.semibold))
                        .foregroundStyle(AppTheme.danger)
                }

                Button {
                    Task { await importCards() }
                } label: {
                    if sessionStore.isCreatingVocab {
                        ProgressView()
                            .frame(maxWidth: .infinity)
                    } else {
                        Text("Import \(importCandidates.count) \(importCandidates.count == 1 ? "Card" : "Cards")")
                    }
                }
                .readingPrimaryButton()
                .disabled(sessionStore.isCreatingVocab || importCandidates.isEmpty)
            }
            .readingCard()
        }
    }

    private var autocompleteDisabled: Bool {
        sessionStore.isAutocompletingVocab || sessionStore.isCreatingVocab || importCandidates.isEmpty
    }

    @ViewBuilder
    private var previewSection: some View {
        if !importCandidates.isEmpty {
            VStack(alignment: .leading, spacing: 10) {
                HStack {
                    Text("Preview")
                        .font(AppTheme.uiFont(size: 16, weight: .bold, relativeTo: .headline))
                        .foregroundStyle(AppTheme.sageDark)
                    Spacer()
                    Text("\(importCandidates.count) \(importCandidates.count == 1 ? "card" : "cards")")
                        .font(.footnote.weight(.semibold))
                        .foregroundStyle(AppTheme.muted)
                }

                ScrollView(.horizontal) {
                    LazyHStack(alignment: .top, spacing: 12) {
                        ForEach(Array(importCandidates.enumerated()), id: \.offset) { _, card in
                            importPreviewCard(card)
                        }
                    }
                    .scrollTargetLayout()
                    .padding(.vertical, 4)
                }
                .scrollIndicators(.hidden)
                .scrollTargetBehavior(.viewAligned)
            }
        }
    }

    private func importPreviewCard(_ card: VocabDraftInput) -> some View {
        VStack(alignment: .leading, spacing: 8) {
            Text(card.term)
                .readingTerm()
            if !card.partOfSpeech.isEmpty {
                Text(card.partOfSpeech.replacingOccurrences(of: "_", with: " "))
                    .font(.caption.weight(.bold))
                    .foregroundStyle(AppTheme.sageDark)
                    .padding(.horizontal, 10)
                    .padding(.vertical, 5)
                    .background(AppTheme.blush.opacity(0.5), in: Capsule())
            }
            if card.meaning.isEmpty {
                Text("Meaning can be added later.")
                    .readingMuted()
            } else {
                Text(card.meaning)
                    .foregroundStyle(AppTheme.ink)
            }
            if !card.chinese.isEmpty {
                Text(card.chinese)
                    .foregroundStyle(AppTheme.clay)
            }
            if !card.exampleSentence.isEmpty {
                Text(card.exampleSentence)
                    .font(.footnote)
                    .readingMuted()
            }
        }
        .padding()
        .frame(width: 228, alignment: .topLeading)
        .frame(minHeight: 172, alignment: .topLeading)
        .background(AppTheme.paper.opacity(0.86), in: RoundedRectangle(cornerRadius: 18, style: .continuous))
        .overlay {
            RoundedRectangle(cornerRadius: 18, style: .continuous)
                .stroke(AppTheme.coral.opacity(0.16), lineWidth: 1)
        }
    }

    private func importCards() async {
        let cards = importCandidates
        let created = await sessionStore.createVocabCards(cards)
        if created == cards.count {
            resetImportedDraft()
            if presentation == .standalone {
                dismiss()
            }
        }
    }

    private func resetImportedDraft() {
        rawText = ""
        parsedCards = []
        enrichedCards = nil
        BulkImportDraftStorage.clear()
    }

    private func autocompleteCards() async {
        guard let cards = await sessionStore.autocompleteVocabCards(parsedCards) else { return }
        enrichedCards = cards
        rawText = formatBulkInput(cards)
    }

    private func parseBulkInput(_ input: String) -> [VocabDraftInput] {
        input
            .split(whereSeparator: \.isNewline)
            .map { String($0).trimmingCharacters(in: .whitespacesAndNewlines) }
            .filter { !$0.isEmpty }
            .map(parseLine)
    }

    private func parseLine(_ line: String) -> VocabDraftInput {
        if line.contains("|") {
            let parts = line.split(separator: "|", omittingEmptySubsequences: false).map {
                String($0).trimmingCharacters(in: .whitespacesAndNewlines)
            }
            return VocabDraftInput(
                term: parts.indices.contains(0) ? parts[0] : "",
                meaning: parts.indices.contains(1) ? parts[1] : "",
                chinese: parts.count >= 5 && parts.indices.contains(2) ? parts[2] : "",
                exampleSentence: parts.count >= 5 && parts.indices.contains(3) ? parts[3] : (parts.indices.contains(2) ? parts[2] : ""),
                partOfSpeech: parts.count >= 5 && parts.indices.contains(4) ? parts[4] : (parts.indices.contains(3) ? parts[3] : ""),
                notes: ""
            )
        }

        let separators = [" - ", "\t", ": ", "："]
        for separator in separators {
            guard let range = line.range(of: separator) else { continue }
            let term = String(line[..<range.lowerBound]).trimmingCharacters(in: .whitespacesAndNewlines)
            let meaning = String(line[range.upperBound...]).trimmingCharacters(in: .whitespacesAndNewlines)
            return VocabDraftInput(term: term, meaning: meaning, exampleSentence: "", notes: "")
        }
        return VocabDraftInput(term: line, meaning: "", exampleSentence: "", notes: "")
    }

    private func formatBulkInput(_ cards: [VocabDraftInput]) -> String {
        cards
            .map { card in
                var fields = [
                    card.term,
                    card.meaning,
                    card.chinese,
                    card.exampleSentence,
                    card.partOfSpeech
                ].map { $0.trimmingCharacters(in: .whitespacesAndNewlines) }
                while fields.count > 1 && fields.last == "" {
                    fields.removeLast()
                }
                return fields.joined(separator: " | ")
            }
            .joined(separator: "\n")
    }

    private func refreshDraftAndSharedCaptures() {
        if rawText.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty {
            rawText = BulkImportDraftStorage.load()
        }
        syncParsedCards(with: rawText)
        loadSharedCaptures()
    }

    private func loadSharedCaptures() {
        let captures = SharedCaptureQueue.load()
        guard !captures.isEmpty else { return }

        let sharedTerms = captures
            .map { $0.term.trimmingCharacters(in: .whitespacesAndNewlines) }
            .filter { !$0.isEmpty }
        guard !sharedTerms.isEmpty else {
            SharedCaptureQueue.clear()
            return
        }

        let existingText = rawText.trimmingCharacters(in: .whitespacesAndNewlines)
        rawText = ([existingText] + sharedTerms)
            .filter { !$0.isEmpty }
            .joined(separator: "\n")
        parsedCards = parseBulkInput(rawText)
        enrichedCards = nil
        BulkImportDraftStorage.save(rawText)
        SharedCaptureQueue.clear()
    }

    private func syncParsedCards(with newValue: String) {
        parsedCards = parseBulkInput(newValue)
        BulkImportDraftStorage.save(newValue)
        if let enrichedCards, formatBulkInput(enrichedCards) == newValue {
            return
        }
        enrichedCards = nil
    }
}

private enum BulkImportDraftStorage {
    private static let key = "bulkImportDraftText"

    static func load() -> String {
        UserDefaults.standard.string(forKey: key) ?? ""
    }

    static func save(_ text: String) {
        let draft = text.trimmingCharacters(in: .whitespacesAndNewlines)
        if draft.isEmpty {
            clear()
        } else {
            UserDefaults.standard.set(text, forKey: key)
        }
    }

    static func clear() {
        UserDefaults.standard.removeObject(forKey: key)
    }
}
