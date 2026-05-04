import SwiftUI

struct ReviewListView: View {
    @EnvironmentObject private var sessionStore: SessionStore
    @State private var isAnswerRevealed = false
    @State private var editingCard: DueCard?
    @State private var deletingCard: DueCard?

    var body: some View {
        Group {
            if sessionStore.isLoadingDueCards && sessionStore.dueCards.isEmpty {
                ProgressView("Loading due cards...")
                    .frame(maxWidth: .infinity, maxHeight: .infinity)
            } else if let card = sessionStore.currentCard {
                ScrollView {
                    VStack(alignment: .leading, spacing: 20) {
                        statusMessages
                        progressHeader(totalDue: sessionStore.dueCards.count)
                        reviewCard(card)
                    }
                    .padding()
                }
                .refreshable {
                    await sessionStore.loadDueCards()
                }
            } else {
                ScrollView {
                    VStack(alignment: .leading, spacing: 20) {
                        statusMessages
                        VStack(alignment: .leading, spacing: 12) {
                            Text("All caught up")
                                .font(.largeTitle.bold())
                            Text("You do not have any cards due right now.")
                                .foregroundStyle(.secondary)
                        }

                        if sessionStore.isLoadingDueCards {
                            ProgressView("Refreshing...")
                        } else {
                            Button("Refresh") {
                                Task { await sessionStore.loadDueCards() }
                            }
                            .buttonStyle(.borderedProminent)
                        }
                    }
                    .padding()
                }
                .refreshable {
                    await sessionStore.loadDueCards()
                }
            }
        }
        .navigationTitle("Due Review")
        .sheet(item: $editingCard) { card in
            AddVocabView(item: card.item)
                .environmentObject(sessionStore)
        }
        .alert("Delete card?", isPresented: deleteConfirmationPresented, presenting: deletingCard) { card in
            Button("Delete", role: .destructive) {
                Task {
                    _ = await sessionStore.deleteVocab(cardID: card.item.id)
                    deletingCard = nil
                }
            }
            Button("Cancel", role: .cancel) {
                deletingCard = nil
            }
        } message: { card in
            Text("This removes \"\(card.item.term)\" from your review queue.")
        }
        .onChange(of: sessionStore.currentCard?.id) { _, _ in
            isAnswerRevealed = false
        }
    }

    private var deleteConfirmationPresented: Binding<Bool> {
        Binding(
            get: { deletingCard != nil },
            set: { isPresented in
                if !isPresented {
                    deletingCard = nil
                }
            }
        )
    }

    @ViewBuilder
    private var statusMessages: some View {
        if !sessionStore.errorMessage.isEmpty {
            VStack(alignment: .leading, spacing: 8) {
                Text(sessionStore.errorMessage)
                    .foregroundStyle(.red)
                Button("Dismiss") {
                    sessionStore.clearError()
                }
                .buttonStyle(.bordered)
            }
        }
    }

    private func progressHeader(totalDue: Int) -> some View {
        VStack(alignment: .leading, spacing: 6) {
            Text("Card 1 of \(totalDue)")
                .font(.headline)
            Text(totalDue == 1 ? "1 card due now" : "\(totalDue) cards due now")
                .foregroundStyle(.secondary)
        }
    }

    private func reviewCard(_ card: DueCard) -> some View {
        VStack(alignment: .leading, spacing: 20) {
            VStack(alignment: .leading, spacing: 8) {
                Text(card.item.term)
                    .font(.largeTitle.bold())
                if isAnswerRevealed {
                    Text(card.item.meaning.isEmpty ? "Meaning not added yet." : card.item.meaning)
                        .font(.title3)
                    if !card.item.example_sentence.isEmpty {
                        Text(card.item.example_sentence)
                            .foregroundStyle(.secondary)
                    }
                    if !card.item.notes.isEmpty {
                        Text(card.item.notes)
                            .padding(.top, 4)
                    }
                } else {
                    Text("Recall the meaning before revealing the answer.")
                        .foregroundStyle(.secondary)
                }
            }

            if isAnswerRevealed {
                VStack(alignment: .leading, spacing: 12) {
                    HStack {
                        Button("Edit") {
                            editingCard = card
                        }
                        .buttonStyle(.bordered)
                        .disabled(sessionStore.isGrading || sessionStore.isDeletingVocab)

                        Button("Delete", role: .destructive) {
                            deletingCard = card
                        }
                        .buttonStyle(.bordered)
                        .disabled(sessionStore.isGrading || sessionStore.isDeletingVocab)
                    }

                    if sessionStore.isDeletingVocab {
                        ProgressView("Deleting card...")
                    }

                    if sessionStore.isGrading {
                        ProgressView("Submitting grade...")
                    }

                    HStack {
                        ForEach(["again", "hard", "good", "easy"], id: \.self) { grade in
                            Button(grade.capitalized) {
                                Task { await sessionStore.grade(cardID: card.item.id, grade: grade) }
                            }
                            .buttonStyle(.borderedProminent)
                            .disabled(sessionStore.isGrading || sessionStore.isLoadingDueCards)
                        }
                    }
                }
            } else {
                Button("Show answer") {
                    isAnswerRevealed = true
                }
                .buttonStyle(.borderedProminent)
                .disabled(sessionStore.isGrading || sessionStore.isLoadingDueCards)
            }
        }
        .padding()
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(.thinMaterial, in: RoundedRectangle(cornerRadius: 20, style: .continuous))
    }
}
