import SwiftUI

struct AddVocabView: View {
    @EnvironmentObject private var sessionStore: SessionStore
    @Environment(\.dismiss) private var dismiss
    private let existingItem: VocabItem?
    @FocusState private var focusedField: Field?
    @State private var term = ""
    @State private var meaning = ""
    @State private var exampleSentence = ""
    @State private var notes = ""
    @State private var detailsExpanded = false
    @State private var lastSavedTerm = ""

    private enum Field {
        case term
    }

    init(item: VocabItem? = nil) {
        self.existingItem = item
        _term = State(initialValue: item?.term ?? "")
        _meaning = State(initialValue: item?.meaning ?? "")
        _exampleSentence = State(initialValue: item?.example_sentence ?? "")
        _notes = State(initialValue: item?.notes ?? "")
    }

    var body: some View {
        NavigationStack {
            ZStack {
                ReadingDeskBackground()

                ScrollView {
                    VStack(alignment: .leading, spacing: 18) {
                        VStack(alignment: .leading, spacing: 8) {
                            Text(existingItem == nil ? "Add a card" : "Edit card")
                                .readingTitle()
                            Text(existingItem == nil ? "Only the term is required. Add details now, or keep moving." : "Keep the wording plain and useful for review later.")
                                .readingMuted()
                        }

                        VStack(alignment: .leading, spacing: 12) {
                            TextField("Term", text: $term)
                                .textInputAutocapitalization(.never)
                                .autocorrectionDisabled()
                                .textFieldStyle(.roundedBorder)
                                .focused($focusedField, equals: .term)
                            DisclosureGroup("Meaning, example, and notes", isExpanded: $detailsExpanded) {
                                VStack(alignment: .leading, spacing: 12) {
                                    TextField("Meaning", text: $meaning, axis: .vertical)
                                        .lineLimit(2...4)
                                        .textFieldStyle(.roundedBorder)
                                    TextField("Example sentence", text: $exampleSentence, axis: .vertical)
                                        .lineLimit(2...4)
                                        .textFieldStyle(.roundedBorder)
                                    TextField("Notes", text: $notes, axis: .vertical)
                                        .lineLimit(2...4)
                                        .textFieldStyle(.roundedBorder)
                                }
                                .padding(.top, 8)
                            }
                            .tint(AppTheme.sage)

                            if existingItem == nil {
                                Button {
                                    Task { await saveAndAddAnother() }
                                } label: {
                                    if sessionStore.isCreatingVocab {
                                        ProgressView()
                                            .frame(maxWidth: .infinity)
                                    } else {
                                        Text("Save + Add Another")
                                            .frame(maxWidth: .infinity)
                                    }
                                }
                                .buttonStyle(.borderedProminent)
                                .disabled(sessionStore.isCreatingVocab || trimmedTerm.isEmpty)
                            }
                        }
                        .readingCard()

                        if !lastSavedTerm.isEmpty {
                            Text("Saved \"\(lastSavedTerm)\". Ready for the next one.")
                                .foregroundStyle(AppTheme.sageDark)
                                .readingCard()
                        }

                        if !sessionStore.errorMessage.isEmpty {
                            Text(sessionStore.errorMessage)
                                .foregroundStyle(AppTheme.danger)
                                .readingCard()
                        }
                    }
                    .padding()
                }
            }
            .navigationTitle(existingItem == nil ? "Add Card" : "Edit Card")
            .toolbar {
                ToolbarItem(placement: .cancellationAction) {
                    Button("Cancel") {
                        dismiss()
                    }
                    .disabled(sessionStore.isCreatingVocab)
                }

                ToolbarItem(placement: .confirmationAction) {
                    Button {
                        Task { await saveAndClose() }
                    } label: {
                        if sessionStore.isCreatingVocab {
                            ProgressView()
                        } else {
                            Text("Save")
                        }
                    }
                    .disabled(sessionStore.isCreatingVocab || trimmedTerm.isEmpty)
                }
            }
            .task {
                focusedField = .term
            }
        }
    }

    private var trimmedTerm: String {
        term.trimmingCharacters(in: .whitespacesAndNewlines)
    }

    private func saveAndClose() async {
        let saved: Bool
        if let existingItem {
            saved = await sessionStore.updateVocab(
                cardID: existingItem.id,
                term: term,
                meaning: meaning,
                exampleSentence: exampleSentence,
                notes: notes
            )
        } else {
            saved = await sessionStore.createVocab(
                term: term,
                meaning: meaning,
                exampleSentence: exampleSentence,
                notes: notes
            )
        }
        if saved {
            dismiss()
        }
    }

    private func saveAndAddAnother() async {
        let savedTerm = trimmedTerm
        let saved = await sessionStore.createVocab(
            term: term,
            meaning: meaning,
            exampleSentence: exampleSentence,
            notes: notes
        )
        guard saved else { return }
        lastSavedTerm = savedTerm
        term = ""
        meaning = ""
        exampleSentence = ""
        notes = ""
        detailsExpanded = false
        focusedField = .term
    }
}
