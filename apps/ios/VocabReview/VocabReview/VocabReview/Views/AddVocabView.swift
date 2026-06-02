import SwiftUI

struct AddVocabView: View {
    @EnvironmentObject private var sessionStore: SessionStore
    @Environment(\.dismiss) private var dismiss
    private let existingItem: VocabItem?
    private let presentation: AddCardsPresentation
    @State private var term = ""
    @State private var meaning = ""
    @State private var chinese = ""
    @State private var exampleSentence = ""
    @State private var notes = ""
    @FocusState private var focusedField: Field?

    private enum Field: Hashable {
        case term
        case meaning
        case chinese
        case exampleSentence
        case notes
    }

    init(item: VocabItem? = nil, presentation: AddCardsPresentation = .standalone) {
        self.existingItem = item
        self.presentation = presentation
        _term = State(initialValue: item?.term ?? "")
        _meaning = State(initialValue: item?.meaning ?? "")
        _chinese = State(initialValue: item?.chinese ?? "")
        _exampleSentence = State(initialValue: item?.example_sentence ?? "")
        _notes = State(initialValue: item?.notes ?? "")
    }

    var body: some View {
        switch presentation {
        case .standalone:
            NavigationStack {
                ZStack {
                    ReadingDeskBackground()

                    ScrollView {
                        formContent(showHeader: true)
                            .padding()
                    }
                }
                .toolbar(.hidden, for: .navigationBar)
                .dismissKeyboardOnTapOutside()
            }
        case .embedded:
            formContent(showHeader: false)
        }
    }

    private func formContent(showHeader: Bool) -> some View {
        VStack(alignment: .leading, spacing: 18) {
            if showHeader {
                VStack(alignment: .leading, spacing: 8) {
                    Text(existingItem == nil ? "Add a card" : "Edit card")
                        .readingTitle()
                    Text(existingItem == nil ? "Only the term is required. Add details now, or keep moving." : "Keep the wording plain and useful for review later.")
                        .readingMuted()
                }
            }

            VStack(alignment: .leading, spacing: 12) {
                Text("Word")
                    .font(.caption.weight(.bold))
                    .textCase(.uppercase)
                    .foregroundStyle(AppTheme.sageDark)
                TextField("e.g. meticulous", text: $term)
                    .textInputAutocapitalization(.never)
                    .autocorrectionDisabled()
                    .readingInputField()
                    .focused($focusedField, equals: .term)

                HStack(alignment: .top, spacing: 12) {
                    VStack(alignment: .leading, spacing: 6) {
                        Text("Meaning")
                            .font(.caption.weight(.bold))
                            .textCase(.uppercase)
                            .foregroundStyle(AppTheme.sageDark)
                        TextField("Short definition", text: $meaning, axis: .vertical)
                            .lineLimit(2...4)
                            .readingInputField()
                            .focused($focusedField, equals: .meaning)
                    }

                    VStack(alignment: .leading, spacing: 6) {
                        Text("Chinese")
                            .font(.caption.weight(.bold))
                            .textCase(.uppercase)
                            .foregroundStyle(AppTheme.sageDark)
                        TextField("中文意思", text: $chinese, axis: .vertical)
                            .lineLimit(2...4)
                            .readingInputField()
                            .focused($focusedField, equals: .chinese)
                    }
                }

                VStack(alignment: .leading, spacing: 6) {
                    Text("Example sentence")
                        .font(.caption.weight(.bold))
                        .textCase(.uppercase)
                        .foregroundStyle(AppTheme.sageDark)
                    TextField("Use it in context", text: $exampleSentence, axis: .vertical)
                        .lineLimit(2...4)
                        .readingInputField()
                        .focused($focusedField, equals: .exampleSentence)
                }

                Text("Notes")
                    .font(.caption.weight(.bold))
                    .textCase(.uppercase)
                    .foregroundStyle(AppTheme.sageDark)
                TextField("Memory hint or source", text: $notes, axis: .vertical)
                    .lineLimit(2...4)
                    .readingInputField()
                    .focused($focusedField, equals: .notes)

                Button {
                    Task {
                        if existingItem == nil {
                            await saveAndAddAnother()
                        } else {
                            await saveAndClose()
                        }
                    }
                } label: {
                    if sessionStore.isCreatingVocab {
                        ProgressView()
                            .frame(maxWidth: .infinity)
                    } else {
                        Text("Save")
                    }
                }
                .readingPrimaryButton()
                .disabled(sessionStore.isCreatingVocab || trimmedTerm.isEmpty)
            }
            .readingCard()

            if !sessionStore.errorMessage.isEmpty {
                Text(sessionStore.errorMessage)
                    .foregroundStyle(AppTheme.danger)
                    .readingCard()
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
                chinese: chinese,
                exampleSentence: exampleSentence,
                notes: notes
            )
        } else {
            saved = await sessionStore.createVocab(
                term: term,
                meaning: meaning,
                chinese: chinese,
                exampleSentence: exampleSentence,
                notes: notes
            )
        }
        if saved {
            dismiss()
        }
    }

    private func saveAndAddAnother() async {
        let saved = await sessionStore.createVocab(
            term: term,
            meaning: meaning,
            chinese: chinese,
            exampleSentence: exampleSentence,
            notes: notes
        )
        guard saved else { return }
        term = ""
        meaning = ""
        chinese = ""
        exampleSentence = ""
        notes = ""
    }
}
