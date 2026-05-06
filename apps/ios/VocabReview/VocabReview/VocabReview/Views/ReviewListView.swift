import SwiftUI

struct ReviewListView: View {
    @EnvironmentObject private var sessionStore: SessionStore
    @State private var isAnswerRevealed = false
    @State private var sessionDeck: [DueCard] = []
    @State private var sessionIndex = 0
    @State private var sessionAgainCount = 0
    @State private var sessionSummary: ReviewSessionSummary?
    @State private var editingCard: DueCard?
    @State private var deletingCard: DueCard?

    private var currentSessionCard: DueCard? {
        guard sessionDeck.indices.contains(sessionIndex) else { return nil }
        return sessionDeck[sessionIndex]
    }

    private var isSessionActive: Bool {
        currentSessionCard != nil
    }

    var body: some View {
        Group {
            if sessionStore.isLoadingDueCards && sessionStore.dueCards.isEmpty {
                ZStack {
                    ReadingDeskBackground()
                    ProgressView("Loading due cards...")
                        .padding()
                        .readingCard()
                }
            } else if let card = currentSessionCard ?? sessionStore.currentCard {
                ZStack {
                    ReadingDeskBackground()
                    ScrollView {
                        VStack(alignment: .leading, spacing: 20) {
                            statusMessages
                            if let summary = sessionSummary, !isSessionActive {
                                sessionSummaryCard(summary)
                            }
                            progressHeader(totalDue: isSessionActive ? sessionDeck.count : sessionStore.dueCards.count)
                            if isSessionActive {
                                sessionCard(card)
                            } else {
                                startSessionCard()
                                reviewCard(card)
                            }
                        }
                        .padding()
                    }
                    .refreshable {
                        await sessionStore.loadDueCards()
                    }
                }
            } else {
                ZStack {
                    ReadingDeskBackground()
                    ScrollView {
                        VStack(alignment: .leading, spacing: 20) {
                            statusMessages
                            if let summary = sessionSummary {
                                sessionSummaryCard(summary)
                            }
                            VStack(alignment: .leading, spacing: 12) {
                                Text("All caught up")
                                    .readingTitle()
                                Text("You do not have any cards due right now.")
                                    .readingMuted()
                            }
                            .readingCard()

                            if sessionStore.isLoadingDueCards {
                                ProgressView("Refreshing...")
                                    .padding()
                                    .readingCard()
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
            if !isSessionActive {
                isAnswerRevealed = false
            }
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
            Text(isSessionActive ? "Card \(sessionIndex + 1) of \(totalDue)" : "Card 1 of \(totalDue)")
                .font(.headline)
                .foregroundStyle(AppTheme.sageDark)
            Text(totalDue == 1 ? "1 card due now" : "\(totalDue) cards due now")
                .readingMuted()
        }
        .readingCard()
    }

    private func startSessionCard() -> some View {
        VStack(alignment: .leading, spacing: 12) {
            Text("Focused session")
                .font(.headline)
                .foregroundStyle(AppTheme.sageDark)
            Text("Review one card at a time. Reveal the answer, grade it, then move to the next card.")
                .readingMuted()
            Button("Start session") {
                startSession()
            }
            .buttonStyle(.borderedProminent)
            .disabled(sessionStore.dueCards.isEmpty || sessionStore.isLoadingDueCards)
        }
        .readingCard()
    }

    private func sessionCard(_ card: DueCard) -> some View {
        VStack(alignment: .leading, spacing: 20) {
            HStack {
                Text("Session in progress")
                    .font(.caption.weight(.bold))
                    .textCase(.uppercase)
                    .foregroundStyle(AppTheme.sageDark)
                Spacer()
                Button("End") {
                    endSession()
                }
                .buttonStyle(.bordered)
                .disabled(sessionStore.isGrading)
            }

            ProgressView(value: Double(sessionIndex), total: Double(max(sessionDeck.count, 1)))
                .tint(AppTheme.clay)

            reviewCardContent(card)
        }
        .padding()
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(
            LinearGradient(
                colors: [AppTheme.paper.opacity(0.96), AppTheme.linen.opacity(0.84)],
                startPoint: .topLeading,
                endPoint: .bottomTrailing
            ),
            in: RoundedRectangle(cornerRadius: 30, style: .continuous)
        )
        .overlay {
            RoundedRectangle(cornerRadius: 30, style: .continuous)
                .stroke(AppTheme.sage.opacity(0.18), lineWidth: 1)
        }
        .shadow(color: AppTheme.ink.opacity(0.1), radius: 26, x: 0, y: 16)
    }

    private func sessionSummaryCard(_ summary: ReviewSessionSummary) -> some View {
        VStack(alignment: .leading, spacing: 8) {
            Text("Session complete")
                .font(.headline)
                .foregroundStyle(AppTheme.sageDark)
            Text("Reviewed \(summary.reviewed) \(summary.reviewed == 1 ? "card" : "cards"). \(summary.again) marked again.")
                .readingMuted()
        }
        .readingCard()
    }

    private func reviewCard(_ card: DueCard) -> some View {
        reviewCardContent(card)
            .padding()
            .frame(maxWidth: .infinity, alignment: .leading)
            .background(AppTheme.paper.opacity(0.9), in: RoundedRectangle(cornerRadius: 28, style: .continuous))
            .overlay {
                RoundedRectangle(cornerRadius: 28, style: .continuous)
                    .stroke(AppTheme.ink.opacity(0.08), lineWidth: 1)
            }
            .shadow(color: AppTheme.ink.opacity(0.1), radius: 26, x: 0, y: 16)
    }

    private func reviewCardContent(_ card: DueCard) -> some View {
        VStack(alignment: .leading, spacing: 20) {
            VStack(alignment: .leading, spacing: 8) {
                Text(card.item.term)
                    .font(.system(.largeTitle, design: .serif, weight: .semibold))
                    .foregroundStyle(AppTheme.ink)
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
                        .readingMuted()
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
                                Task {
                                    if isSessionActive {
                                        await gradeSessionCard(card, grade: grade)
                                    } else {
                                        await sessionStore.grade(cardID: card.item.id, grade: grade)
                                    }
                                }
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
    }

    private func startSession() {
        guard !sessionStore.dueCards.isEmpty else { return }
        sessionDeck = sessionStore.dueCards
        sessionIndex = 0
        sessionAgainCount = 0
        sessionSummary = nil
        isAnswerRevealed = false
    }

    private func endSession() {
        sessionDeck = []
        sessionIndex = 0
        sessionAgainCount = 0
        isAnswerRevealed = false
    }

    private func gradeSessionCard(_ card: DueCard, grade: String) async {
        await sessionStore.grade(cardID: card.item.id, grade: grade)
        guard sessionStore.errorMessage.isEmpty else { return }

        let reviewed = sessionIndex + 1
        let again = sessionAgainCount + (grade == "again" ? 1 : 0)
        sessionAgainCount = again

        if reviewed >= sessionDeck.count {
            sessionSummary = ReviewSessionSummary(reviewed: reviewed, again: again)
            endSession()
        } else {
            sessionIndex = reviewed
            isAnswerRevealed = false
        }
    }
}

private struct ReviewSessionSummary {
    let reviewed: Int
    let again: Int
}
