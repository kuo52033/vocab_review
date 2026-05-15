import SwiftUI

struct ReviewListView: View {
    @EnvironmentObject private var sessionStore: SessionStore
    @State private var sessionDeck: [QuizCard] = []
    @State private var sessionIndex = 0
    @State private var selectedOptionID = ""
    @State private var correctCount = 0
    @State private var wrongCount = 0
    @State private var pendingNextDue = ""
    @State private var sessionSummary: ReviewSessionSummary?
    @State private var isStartingReview = false

    private let sessionLimit = 12

    private var currentQuizCard: QuizCard? {
        guard sessionDeck.indices.contains(sessionIndex) else { return nil }
        return sessionDeck[sessionIndex]
    }

    private var isSessionActive: Bool {
        currentQuizCard != nil
    }

    var body: some View {
        ZStack {
            ReadingDeskBackground()

            ScrollView {
                VStack(alignment: .leading, spacing: 18) {
                    statusMessages

                    VStack(alignment: .leading, spacing: 8) {
                        Text("Start Review")
                            .readingTitle()
                        Text("Answer one card at a time. Each session uses up to \(sessionLimit) due words.")
                            .readingMuted()
                    }

                    if let card = currentQuizCard {
                        quizCard(card)
                    } else {
                        startReviewCard
                    }

                    if let summary = sessionSummary {
                        sessionSummaryCard(summary)
                    }
                }
                .padding()
            }
            .refreshable {
                await sessionStore.loadDueCards()
                await sessionStore.loadReviewStats()
            }
        }
        .navigationTitle("Start Review")
    }

    @ViewBuilder
    private var statusMessages: some View {
        if !sessionStore.errorMessage.isEmpty {
            VStack(alignment: .leading, spacing: 8) {
                Text(sessionStore.errorMessage)
                    .foregroundStyle(AppTheme.danger)
                Button("Dismiss") {
                    sessionStore.clearError()
                }
                .buttonStyle(.bordered)
            }
            .readingCard()
        }
    }

    private var startReviewCard: some View {
        VStack(alignment: .leading, spacing: 18) {
            HStack {
                Text("Quiz Mode")
                    .font(.caption.weight(.bold))
                    .textCase(.uppercase)
                    .foregroundStyle(AppTheme.sageDark)
                Spacer()
                Text("\(sessionStore.reviewStats.due_now) due now")
                    .font(.caption.weight(.semibold))
                    .foregroundStyle(AppTheme.clay)
                    .padding(.horizontal, 10)
                    .padding(.vertical, 6)
                    .background(AppTheme.clay.opacity(0.12), in: Capsule())
            }

            VStack(alignment: .leading, spacing: 6) {
                Text(sessionStore.reviewStats.due_now == 0 ? "Clear desk." : "Ready when you are.")
                    .font(.system(.largeTitle, design: .serif, weight: .semibold))
                    .foregroundStyle(AppTheme.ink)
                Text(sessionStore.reviewStats.due_now == 0 ? "No due cards right now." : "A short multiple-choice sprint is waiting.")
                    .readingMuted()
            }

            Button {
                Task { await startReviewSession() }
            } label: {
                if isStartingReview {
                    ProgressView()
                        .frame(maxWidth: .infinity)
                } else {
                    Text(sessionStore.reviewStats.due_now == 0 ? "All caught up" : "Start Review")
                        .frame(maxWidth: .infinity)
                }
            }
            .buttonStyle(.borderedProminent)
            .disabled(sessionStore.dueCards.isEmpty || isStartingReview)
        }
        .padding()
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(
            LinearGradient(
                colors: [AppTheme.paper.opacity(0.96), AppTheme.linen.opacity(0.78)],
                startPoint: .topLeading,
                endPoint: .bottomTrailing
            ),
            in: RoundedRectangle(cornerRadius: 28, style: .continuous)
        )
        .overlay {
            RoundedRectangle(cornerRadius: 28, style: .continuous)
                .stroke(AppTheme.ink.opacity(0.08), lineWidth: 1)
        }
        .shadow(color: AppTheme.ink.opacity(0.1), radius: 24, x: 0, y: 14)
    }

    private func quizCard(_ quizCard: QuizCard) -> some View {
        VStack(alignment: .leading, spacing: 18) {
            HStack {
                Text("Card \(sessionIndex + 1) of \(sessionDeck.count)")
                    .font(.caption.weight(.bold))
                    .textCase(.uppercase)
                    .foregroundStyle(AppTheme.sageDark)
                Spacer()
                Button("End") {
                    endReviewSession()
                }
                .buttonStyle(.bordered)
                .disabled(sessionStore.isGrading)
            }

            ProgressView(value: Double(sessionIndex), total: Double(max(sessionDeck.count, 1)))
                .tint(AppTheme.clay)

            VStack(alignment: .leading, spacing: 8) {
                Text("Word")
                    .font(.caption.weight(.bold))
                    .textCase(.uppercase)
                    .foregroundStyle(AppTheme.sageDark)
                Text(quizCard.card.item.term)
                    .font(.system(size: 46, weight: .semibold, design: .serif))
                    .foregroundStyle(AppTheme.ink)
                Text("Choose the correct meaning.")
                    .readingMuted()
            }

            VStack(spacing: 10) {
                ForEach(Array(quizCard.options.enumerated()), id: \.element.id) { index, option in
                    answerOptionButton(option, index: index)
                }
            }

            quizFeedback(for: quizCard)

            if !selectedOptionID.isEmpty {
                Button {
                    advanceQuizCard()
                } label: {
                    Text(sessionIndex + 1 >= sessionDeck.count ? "Show summary" : "Next word")
                        .frame(maxWidth: .infinity)
                }
                .buttonStyle(.borderedProminent)
                .disabled(sessionStore.isGrading)
            }
        }
        .padding()
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(AppTheme.paper.opacity(0.94), in: RoundedRectangle(cornerRadius: 28, style: .continuous))
        .overlay {
            RoundedRectangle(cornerRadius: 28, style: .continuous)
                .stroke(AppTheme.ink.opacity(0.08), lineWidth: 1)
        }
        .shadow(color: AppTheme.ink.opacity(0.1), radius: 24, x: 0, y: 14)
    }

    private func answerOptionButton(_ option: QuizOption, index: Int) -> some View {
        let isSelected = selectedOptionID == option.id
        let showCorrect = !selectedOptionID.isEmpty && option.isCorrect
        let showWrong = isSelected && !option.isCorrect

        return Button {
            Task { await selectAnswer(option) }
        } label: {
            HStack(alignment: .top, spacing: 12) {
                Text(String(UnicodeScalar(65 + index)!))
                    .font(.caption.weight(.bold))
                    .foregroundStyle(AppTheme.paper)
                    .frame(width: 28, height: 28)
                    .background(optionBadgeColor(showCorrect: showCorrect, showWrong: showWrong), in: Circle())
                Text(option.text)
                    .multilineTextAlignment(.leading)
                    .foregroundStyle(AppTheme.ink)
                Spacer()
            }
            .padding()
            .frame(maxWidth: .infinity, alignment: .leading)
            .background(optionBackground(showCorrect: showCorrect, showWrong: showWrong), in: RoundedRectangle(cornerRadius: 18, style: .continuous))
            .overlay {
                RoundedRectangle(cornerRadius: 18, style: .continuous)
                    .stroke(optionBorder(showCorrect: showCorrect, showWrong: showWrong), lineWidth: 1)
            }
        }
        .buttonStyle(.plain)
        .disabled(!selectedOptionID.isEmpty || sessionStore.isGrading)
    }

    private func quizFeedback(for quizCard: QuizCard) -> some View {
        Group {
            if selectedOptionID.isEmpty {
                Text("Correct answers use easy. Wrong answers use again.")
                    .readingMuted()
            } else if quizCard.options.first(where: { $0.id == selectedOptionID })?.isCorrect == true {
                Text("Correct. This card will move further out.")
                    .foregroundStyle(AppTheme.sageDark)
            } else {
                Text("Not this one. The card will return sooner.")
                    .foregroundStyle(AppTheme.danger)
            }
        }
        .font(.callout.weight(.semibold))
    }

    private func sessionSummaryCard(_ summary: ReviewSessionSummary) -> some View {
        HStack(alignment: .top, spacing: 16) {
            VStack(alignment: .leading, spacing: 6) {
                Text("Review complete.")
                    .font(.headline)
                    .foregroundStyle(AppTheme.sageDark)
                Text("\(summary.correct) correct, \(summary.wrong) wrong.")
                    .readingMuted()
                if let lastNextDue = summary.lastNextDue {
                    Text("Last card returns \(formattedDate(lastNextDue)).")
                        .font(.caption)
                        .readingMuted()
                }
            }
            Spacer()
            VStack(alignment: .trailing, spacing: 2) {
                Text("\(summary.accuracy)%")
                    .font(.title3.weight(.bold))
                    .foregroundStyle(AppTheme.sageDark)
                Text("accuracy")
                    .font(.caption)
                    .readingMuted()
            }
        }
        .readingCard()
    }

    private func optionBackground(showCorrect: Bool, showWrong: Bool) -> Color {
        if showCorrect { return AppTheme.sage.opacity(0.18) }
        if showWrong { return AppTheme.danger.opacity(0.12) }
        return AppTheme.paper.opacity(0.82)
    }

    private func optionBorder(showCorrect: Bool, showWrong: Bool) -> Color {
        if showCorrect { return AppTheme.sage.opacity(0.45) }
        if showWrong { return AppTheme.danger.opacity(0.45) }
        return AppTheme.ink.opacity(0.08)
    }

    private func optionBadgeColor(showCorrect: Bool, showWrong: Bool) -> Color {
        if showCorrect { return AppTheme.sage }
        if showWrong { return AppTheme.danger }
        return AppTheme.muted
    }

    private func startReviewSession() async {
        guard !sessionStore.dueCards.isEmpty else { return }
        isStartingReview = true
        defer { isStartingReview = false }

        let candidates = await sessionStore.loadReviewOptionCards()
        let deck = buildQuizDeck(
            dueCards: sessionStore.dueCards,
            candidates: candidates,
            limit: sessionLimit
        )

        guard !deck.isEmpty else {
            sessionStore.errorMessage = "Start Review needs at least one due card with a meaning and one other active card with a meaning."
            return
        }

        sessionDeck = deck
        sessionIndex = 0
        selectedOptionID = ""
        correctCount = 0
        wrongCount = 0
        pendingNextDue = ""
        sessionSummary = nil
        sessionStore.clearError()
    }

    private func selectAnswer(_ option: QuizOption) async {
        guard let currentQuizCard, selectedOptionID.isEmpty else { return }
        selectedOptionID = option.id
        let grade = option.isCorrect ? "easy" : "again"
        guard let nextDue = await sessionStore.gradeAndReturnNextDue(cardID: currentQuizCard.card.item.id, grade: grade) else {
            selectedOptionID = ""
            return
        }
        correctCount += option.isCorrect ? 1 : 0
        wrongCount += option.isCorrect ? 0 : 1
        pendingNextDue = nextDue
        if option.isCorrect {
            advanceQuizCard()
        }
    }

    private func advanceQuizCard() {
        let reviewed = sessionIndex + 1
        if reviewed >= sessionDeck.count {
            sessionSummary = ReviewSessionSummary(
                reviewed: reviewed,
                correct: correctCount,
                wrong: wrongCount,
                lastNextDue: pendingNextDue.isEmpty ? nil : pendingNextDue
            )
            endReviewSession()
            return
        }

        sessionIndex = reviewed
        selectedOptionID = ""
        pendingNextDue = ""
    }

    private func endReviewSession() {
        sessionDeck = []
        sessionIndex = 0
        selectedOptionID = ""
        pendingNextDue = ""
    }

    private func formattedDate(_ value: String) -> String {
        guard let date = ISO8601DateFormatter.vocabReview.date(from: value) else {
            return value
        }
        return date.formatted(date: .abbreviated, time: .shortened)
    }
}

private struct QuizCard {
    let card: DueCard
    let options: [QuizOption]
}

private struct QuizOption: Identifiable {
    let id: String
    let text: String
    let isCorrect: Bool
}

private struct ReviewSessionSummary {
    let reviewed: Int
    let correct: Int
    let wrong: Int
    let lastNextDue: String?

    var accuracy: Int {
        Int((Double(correct) / Double(max(reviewed, 1)) * 100).rounded())
    }
}

private func buildQuizDeck(dueCards: [DueCard], candidates: [DueCard], limit: Int) -> [QuizCard] {
    let cardsWithAnswers = dueCards.filter { !$0.item.meaning.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty }
    let candidateAnswers = candidates
        .filter { !$0.item.meaning.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty }
        .map { (id: $0.item.id, text: $0.item.meaning.trimmingCharacters(in: .whitespacesAndNewlines)) }

    return cardsWithAnswers.shuffled()
        .prefix(limit)
        .compactMap { card in
            let correctText = card.item.meaning.trimmingCharacters(in: .whitespacesAndNewlines)
            let distractors = candidateAnswers
                .filter { $0.id != card.item.id && $0.text != correctText }
                .shuffled()
                .prefix(3)

            let options = ([QuizOption(id: "\(card.item.id)-correct", text: correctText, isCorrect: true)] + distractors.map {
                QuizOption(id: "\(card.item.id)-\($0.id)", text: $0.text, isCorrect: false)
            }).shuffled()

            guard options.count >= 2 else { return nil }
            return QuizCard(card: card, options: options)
        }
}
