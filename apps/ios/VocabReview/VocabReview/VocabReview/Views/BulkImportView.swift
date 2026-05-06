import SwiftUI

struct BulkImportView: View {
    @EnvironmentObject private var sessionStore: SessionStore
    @Environment(\.dismiss) private var dismiss
    @State private var rawText = ""
    @State private var sharedCaptures: [SharedQueuedCapture] = []

    private var parsedCards: [VocabDraftInput] {
        parseBulkInput(rawText)
    }

    private var sharedCards: [VocabDraftInput] {
        sharedCaptures.map {
            VocabDraftInput(term: $0.term, meaning: "", exampleSentence: "", notes: sourceNote(for: $0))
        }
    }

    private var importCandidates: [VocabDraftInput] {
        sharedCards + parsedCards
    }

    var body: some View {
        NavigationStack {
            ZStack {
                ReadingDeskBackground()

                ScrollView {
                    VStack(alignment: .leading, spacing: 18) {
                        VStack(alignment: .leading, spacing: 8) {
                            Text("Bulk import")
                                .readingTitle()
                            Text("Import shared selections or paste one card per line.")
                                .readingMuted()
                        }

                        sharedQueueSection

                        VStack(alignment: .leading, spacing: 12) {
                            TextEditor(text: $rawText)
                                .frame(minHeight: 190)
                                .scrollContentBackground(.hidden)
                                .padding(10)
                                .background(AppTheme.paper.opacity(0.82), in: RoundedRectangle(cornerRadius: 16, style: .continuous))
                                .overlay {
                                    RoundedRectangle(cornerRadius: 16, style: .continuous)
                                        .stroke(AppTheme.ink.opacity(0.08), lineWidth: 1)
                                }

                            Text("Examples: abandon - to leave behind, meticulous: very careful, take off")
                                .font(.footnote)
                                .readingMuted()
                        }
                        .readingCard()

                        previewSection

                        if !sessionStore.errorMessage.isEmpty {
                            Text(sessionStore.errorMessage)
                                .foregroundStyle(AppTheme.danger)
                                .readingCard()
                        }
                    }
                    .padding()
                }
            }
            .navigationTitle("Import Cards")
            .toolbar {
                ToolbarItem(placement: .cancellationAction) {
                    Button("Cancel") {
                        dismiss()
                    }
                    .disabled(sessionStore.isCreatingVocab)
                }

                ToolbarItem(placement: .confirmationAction) {
                    Button {
                        Task { await importCards() }
                    } label: {
                        if sessionStore.isCreatingVocab {
                            ProgressView()
                        } else {
                            Text("Import")
                        }
                    }
                    .disabled(sessionStore.isCreatingVocab || importCandidates.isEmpty)
                }
            }
            .task {
                sharedCaptures = SharedCaptureQueue.load()
            }
        }
    }

    @ViewBuilder
    private var sharedQueueSection: some View {
        VStack(alignment: .leading, spacing: 12) {
            HStack {
                Text("Shared queue")
                    .font(.headline)
                    .foregroundStyle(AppTheme.sageDark)
                Spacer()
                Text("\(sharedCaptures.count) \(sharedCaptures.count == 1 ? "item" : "items")")
                    .font(.footnote.weight(.semibold))
                    .foregroundStyle(AppTheme.muted)
            }

            if sharedCaptures.isEmpty {
                Text("Select text in Safari or another app, tap Share, then choose Vocab Review.")
                    .readingMuted()
            } else {
                ForEach(sharedCaptures) { capture in
                    VStack(alignment: .leading, spacing: 4) {
                        Text(capture.term)
                            .readingTerm()
                        Text(capture.sourceTitle)
                            .font(.footnote)
                            .readingMuted()
                    }
                    .padding(.vertical, 8)
                    .overlay(alignment: .bottom) {
                        Rectangle()
                            .fill(AppTheme.ink.opacity(0.07))
                            .frame(height: 1)
                    }
                }

                Button("Clear shared queue", role: .destructive) {
                    SharedCaptureQueue.clear()
                    sharedCaptures = []
                }
                .buttonStyle(.bordered)
                .disabled(sessionStore.isCreatingVocab)
            }
        }
        .readingCard()
    }

    @ViewBuilder
    private var previewSection: some View {
        VStack(alignment: .leading, spacing: 12) {
            HStack {
                Text("Preview")
                    .font(.headline)
                    .foregroundStyle(AppTheme.sageDark)
                Spacer()
                Text("\(importCandidates.count) \(importCandidates.count == 1 ? "card" : "cards")")
                    .font(.footnote.weight(.semibold))
                    .foregroundStyle(AppTheme.muted)
            }

            if importCandidates.isEmpty {
                Text("Shared or pasted cards will appear here before import.")
                    .readingMuted()
            } else {
                ForEach(Array(importCandidates.enumerated()), id: \.offset) { _, card in
                    VStack(alignment: .leading, spacing: 4) {
                        Text(card.term)
                            .readingTerm()
                        if card.meaning.isEmpty {
                            Text("Meaning can be added later.")
                                .readingMuted()
                        } else {
                            Text(card.meaning)
                                .foregroundStyle(AppTheme.ink)
                        }
                    }
                    .padding(.vertical, 8)
                    .overlay(alignment: .bottom) {
                        Rectangle()
                            .fill(AppTheme.ink.opacity(0.07))
                            .frame(height: 1)
                    }
                }
            }
        }
        .readingCard()
    }

    private func importCards() async {
        let cards = importCandidates
        let created = await sessionStore.createVocabCards(cards)
        if created == cards.count {
            if !sharedCaptures.isEmpty {
                SharedCaptureQueue.clear()
            }
            dismiss()
        }
    }

    private func sourceNote(for capture: SharedQueuedCapture) -> String {
        if capture.sourceURL.isEmpty {
            return "Shared from iOS"
        }
        return "Shared from \(capture.sourceURL)"
    }

    private func parseBulkInput(_ input: String) -> [VocabDraftInput] {
        input
            .split(whereSeparator: \.isNewline)
            .map { String($0).trimmingCharacters(in: .whitespacesAndNewlines) }
            .filter { !$0.isEmpty }
            .map(parseLine)
    }

    private func parseLine(_ line: String) -> VocabDraftInput {
        let separators = [" - ", "\t", ": ", "："]
        for separator in separators {
            guard let range = line.range(of: separator) else { continue }
            let term = String(line[..<range.lowerBound]).trimmingCharacters(in: .whitespacesAndNewlines)
            let meaning = String(line[range.upperBound...]).trimmingCharacters(in: .whitespacesAndNewlines)
            return VocabDraftInput(term: term, meaning: meaning, exampleSentence: "", notes: "")
        }
        return VocabDraftInput(term: line, meaning: "", exampleSentence: "", notes: "")
    }
}
