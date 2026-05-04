import SwiftUI

struct AddVocabView: View {
    @EnvironmentObject private var sessionStore: SessionStore
    @Environment(\.dismiss) private var dismiss
    @State private var term = ""
    @State private var meaning = ""
    @State private var exampleSentence = ""
    @State private var notes = ""

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
            .navigationTitle("Add Card")
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
                            let created = await sessionStore.createVocab(
                                term: term,
                                meaning: meaning,
                                exampleSentence: exampleSentence,
                                notes: notes
                            )
                            if created {
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
