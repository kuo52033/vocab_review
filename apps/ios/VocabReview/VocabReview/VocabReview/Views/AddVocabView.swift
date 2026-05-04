import SwiftUI

struct AddVocabView: View {
    @EnvironmentObject private var sessionStore: SessionStore
    @Environment(\.dismiss) private var dismiss
    private let existingItem: VocabItem?
    @State private var term = ""
    @State private var meaning = ""
    @State private var exampleSentence = ""
    @State private var notes = ""

    init(item: VocabItem? = nil) {
        self.existingItem = item
        _term = State(initialValue: item?.term ?? "")
        _meaning = State(initialValue: item?.meaning ?? "")
        _exampleSentence = State(initialValue: item?.example_sentence ?? "")
        _notes = State(initialValue: item?.notes ?? "")
    }

    var body: some View {
        NavigationStack {
            Form {
                Section("Card") {
                    TextField("Term", text: $term)
                        .textInputAutocapitalization(.never)
                        .autocorrectionDisabled()
                    TextField("Meaning", text: $meaning, axis: .vertical)
                        .lineLimit(2...4)
                    TextField("Example sentence", text: $exampleSentence, axis: .vertical)
                        .lineLimit(2...4)
                    TextField("Notes", text: $notes, axis: .vertical)
                        .lineLimit(2...4)
                }

                if !sessionStore.errorMessage.isEmpty {
                    Section {
                        Text(sessionStore.errorMessage)
                            .foregroundStyle(.red)
                    }
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
                        Task {
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
                    } label: {
                        if sessionStore.isCreatingVocab {
                            ProgressView()
                        } else {
                            Text("Save")
                        }
                    }
                    .disabled(sessionStore.isCreatingVocab || term.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty)
                }
            }
        }
    }
}
