import SwiftUI

struct BulkImportView: View {
    @EnvironmentObject private var sessionStore: SessionStore
    @Environment(\.dismiss) private var dismiss
    @State private var rawText = ""

    private var parsedCards: [VocabDraftInput] {
        parseBulkInput(rawText)
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
                            Text("Paste one card per line. Use \"term - meaning\" or just the term.")
                                .readingMuted()
                        }

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
                    .disabled(sessionStore.isCreatingVocab || parsedCards.isEmpty)
                }
            }
        }
    }

    @ViewBuilder
    private var previewSection: some View {
        VStack(alignment: .leading, spacing: 12) {
            HStack {
                Text("Preview")
                    .font(.headline)
                    .foregroundStyle(AppTheme.sageDark)
                Spacer()
                Text("\(parsedCards.count) \(parsedCards.count == 1 ? "card" : "cards")")
                    .font(.footnote.weight(.semibold))
                    .foregroundStyle(AppTheme.muted)
            }

            if parsedCards.isEmpty {
                Text("Paste lines above to preview import cards.")
                    .readingMuted()
            } else {
                ForEach(Array(parsedCards.enumerated()), id: \.offset) { _, card in
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
        let created = await sessionStore.createVocabCards(parsedCards)
        if created == parsedCards.count {
            dismiss()
        }
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
