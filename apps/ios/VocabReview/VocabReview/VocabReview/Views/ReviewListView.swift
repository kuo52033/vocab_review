import SwiftUI

struct ReviewListView: View {
    @EnvironmentObject private var sessionStore: SessionStore
    @Binding private var isReviewSessionActive: Bool
    @State private var sessionDeck: [QuizCard] = []
    @State private var sessionIndex = 0
    @State private var selectedOptionID = ""
    @State private var correctCount = 0
    @State private var wrongCount = 0
    @State private var pendingNextDue = ""
    @State private var sessionSummary: ReviewSessionSummary?
    @State private var isStartingReview = false
    @State private var isAdvancingQuizCard = false
    @State private var focusedReviewContentHeight: CGFloat = 0

    private let sessionLimit = 12

    init(isReviewSessionActive: Binding<Bool> = .constant(false)) {
        _isReviewSessionActive = isReviewSessionActive
    }

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

            if let card = currentQuizCard {
                focusedReviewSession(card)
                    .transition(.opacity.combined(with: .scale(scale: 0.98)))
            } else {
                reviewHome
                    .transition(.opacity)
            }
        }
        .navigationTitle(isSessionActive ? "" : "Start Review")
        .animation(.easeOut(duration: 0.28), value: isSessionActive)
        .animation(.easeOut(duration: 0.28), value: sessionIndex)
        .onChange(of: isSessionActive) { _, newValue in
            isReviewSessionActive = newValue
        }
        .onDisappear {
            isReviewSessionActive = false
        }
    }

    private var reviewHome: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 18) {
                VStack(alignment: .leading, spacing: 8) {
                    Text("Start Review")
                        .readingTitle()
                    Text("Answer one card at a time. Each session uses up to \(sessionLimit) due words.")
                        .readingMuted()
                }

                startReviewCard
                    .transition(.opacity.combined(with: .move(edge: .bottom)))

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

    private func focusedReviewSession(_ card: QuizCard) -> some View {
        GeometryReader { proxy in
            ScrollView {
                VStack {
                    quizCard(card)
                        .id(card.card.item.id)
                        .opacity(isAdvancingQuizCard ? 0 : 1)
                        .scaleEffect(isAdvancingQuizCard ? 0.98 : 1)
                        .offset(y: isAdvancingQuizCard ? 18 : 0)
                        .transition(.opacity.combined(with: .move(edge: .bottom)))
                }
                .padding(.horizontal, 18)
                .frame(maxWidth: .infinity, minHeight: proxy.size.height, alignment: .center)
                .background {
                    GeometryReader { contentProxy in
                        Color.clear
                            .preference(key: FocusedReviewContentHeightKey.self, value: contentProxy.size.height)
                    }
                }
            }
            .scrollIndicators(.hidden)
            .scrollBounceBehavior(.basedOnSize)
            .scrollDisabled(focusedReviewContentHeight <= proxy.size.height + 1)
            .onPreferenceChange(FocusedReviewContentHeightKey.self) { focusedReviewContentHeight = $0 }
            .refreshable {
                await sessionStore.loadDueCards()
                await sessionStore.loadReviewStats()
            }
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

            if sessionStore.reviewStats.due_now > 0 {
                Button {
                    Task { await startReviewSession() }
                } label: {
                    if isStartingReview {
                        ProgressView()
                            .frame(maxWidth: .infinity)
                    } else {
                        Text("Start Review")
                            .frame(maxWidth: .infinity)
                    }
                }
                .buttonStyle(.borderedProminent)
                .disabled(sessionStore.dueCards.isEmpty || isStartingReview)
            }
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

            VStack(alignment: .center, spacing: 8) {
                Text(partOfSpeechLabel(quizCard.card.item))
                    .font(.caption.weight(.bold))
                    .textCase(.uppercase)
                    .foregroundStyle(AppTheme.sageDark)
                Text(quizCard.card.item.term)
                    .font(.system(size: 46, weight: .regular, design: .rounded))
                    .foregroundStyle(AppTheme.ink)
                    .multilineTextAlignment(.center)
                    .tracking(0)
            }
            .frame(maxWidth: .infinity, alignment: .center)
            .id("prompt-\(quizCard.card.item.id)")
            .transition(.opacity.combined(with: .move(edge: .bottom)))

            VStack(spacing: 10) {
                ForEach(Array(quizCard.options.enumerated()), id: \.element.id) { index, option in
                    answerOptionButton(option, index: index)
                }
            }
            .id("options-\(quizCard.card.item.id)")
            .transition(.opacity.combined(with: .move(edge: .bottom)))

            quizFeedback(for: quizCard)

            if !selectedOptionID.isEmpty {
                Button {
                    advanceQuizCard()
                } label: {
                    Text(sessionIndex + 1 >= sessionDeck.count ? "Show summary" : "Next word")
                        .frame(maxWidth: .infinity)
                }
                .buttonStyle(.borderedProminent)
                .disabled(sessionStore.isGrading || isAdvancingQuizCard)
            }
        }
        .padding()
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(AppTheme.paper.opacity(0.94), in: RoundedRectangle(cornerRadius: 28, style: .continuous))
        .overlay {
            RoundedRectangle(cornerRadius: 28, style: .continuous)
                .stroke(AppTheme.coral.opacity(0.18), lineWidth: 1)
        }
        .shadow(color: AppTheme.sageDark.opacity(0.12), radius: 24, x: 0, y: 14)
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
                    .foregroundStyle(AppTheme.ink)
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
        .scaleEffect(isSelected ? 0.985 : 1)
        .animation(.easeOut(duration: 0.16), value: selectedOptionID)
    }

    private func quizFeedback(for quizCard: QuizCard) -> some View {
        Group {
            if selectedOptionID.isEmpty {
                EmptyView()
            } else if quizCard.options.first(where: { $0.id == selectedOptionID })?.isCorrect == true {
                resultBanner(
                    title: "Correct",
                    item: quizCard.card.item,
                    correctAnswer: nil,
                    isCorrect: true
                )
            } else {
                resultBanner(
                    title: "Review again",
                    item: quizCard.card.item,
                    correctAnswer: quizCard.card.item.meaning,
                    isCorrect: false
                )
            }
        }
    }

    private func resultBanner(title: String, item: VocabItem, correctAnswer: String?, isCorrect: Bool) -> some View {
        HStack(alignment: .top, spacing: 12) {
            Image(systemName: isCorrect ? "checkmark.circle.fill" : "exclamationmark.circle.fill")
                .font(.title2)
                .foregroundStyle(isCorrect ? AppTheme.successDark : AppTheme.danger)
            VStack(alignment: .leading, spacing: 5) {
                Text(title)
                    .font(.headline)
                    .foregroundStyle(isCorrect ? AppTheme.successDark : AppTheme.danger)
                if !item.chinese.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty {
                    Text(item.chinese)
                        .font(.callout.weight(.semibold))
                        .foregroundStyle(AppTheme.ink)
                }
                if let correctAnswer {
                    Text("Correct answer: \(correctAnswer)")
                        .font(.footnote)
                        .readingMuted()
                }
                if !item.example_sentence.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty {
                    Text(item.example_sentence)
                        .font(.footnote)
                        .readingMuted()
                }
            }
        }
        .padding()
        .frame(maxWidth: .infinity, alignment: .leading)
        .background((isCorrect ? AppTheme.success.opacity(0.18) : AppTheme.danger.opacity(0.1)), in: RoundedRectangle(cornerRadius: 18, style: .continuous))
        .overlay {
            RoundedRectangle(cornerRadius: 18, style: .continuous)
                .stroke((isCorrect ? AppTheme.successDark : AppTheme.danger).opacity(0.42), lineWidth: 2)
        }
        .transition(.opacity.combined(with: .scale(scale: 0.98)))
    }

    private func partOfSpeechLabel(_ item: VocabItem) -> String {
        if item.part_of_speech.isEmpty {
            return "Word"
        }
        return item.part_of_speech.replacingOccurrences(of: "_", with: " ")
    }

    private func sessionSummaryCard(_ summary: ReviewSessionSummary) -> some View {
        VStack(alignment: .leading, spacing: 14) {
            HStack(alignment: .top, spacing: 14) {
                Image(systemName: "checkmark")
                    .font(.title3.weight(.black))
                    .foregroundStyle(AppTheme.paper)
                    .frame(width: 48, height: 48)
                    .background(
                        LinearGradient(
                            colors: [AppTheme.coral, AppTheme.danger],
                            startPoint: .topLeading,
                            endPoint: .bottomTrailing
                        ),
                        in: RoundedRectangle(cornerRadius: 16, style: .continuous)
                    )
                    .shadow(color: AppTheme.danger.opacity(0.18), radius: 14, x: 0, y: 8)

                VStack(alignment: .leading, spacing: 5) {
                    Text("Review complete")
                        .font(.headline.weight(.bold))
                        .textCase(.uppercase)
                        .foregroundStyle(AppTheme.danger)
                    Text("\(summary.reviewed) cards reviewed")
                        .readingMuted()
                }

                Spacer(minLength: 8)

                VStack(alignment: .trailing, spacing: 3) {
                    Text("\(summary.accuracy)%")
                        .font(.system(.title, design: .rounded, weight: .bold))
                        .foregroundStyle(AppTheme.ink)
                    Text("accuracy")
                        .font(.caption.weight(.bold))
                        .textCase(.uppercase)
                        .foregroundStyle(AppTheme.danger)
                }
                .padding(.horizontal, 14)
                .padding(.vertical, 10)
                .background(AppTheme.paper.opacity(0.72), in: RoundedRectangle(cornerRadius: 18, style: .continuous))
                .overlay {
                    RoundedRectangle(cornerRadius: 18, style: .continuous)
                        .stroke(AppTheme.danger.opacity(0.16), lineWidth: 1)
                }
            }

            HStack(spacing: 8) {
                summaryPill("\(summary.correct) correct")
                summaryPill("\(summary.wrong) wrong")
            }

            if let lastNextDue = summary.lastNextDue {
                Text("Last card returns \(formattedDate(lastNextDue)).")
                    .font(.caption)
                    .readingMuted()
            }
        }
        .padding()
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(
            LinearGradient(
                colors: [AppTheme.paper.opacity(0.98), AppTheme.linen.opacity(0.92)],
                startPoint: .topLeading,
                endPoint: .bottomTrailing
            ),
            in: RoundedRectangle(cornerRadius: 24, style: .continuous)
        )
        .overlay {
            RoundedRectangle(cornerRadius: 24, style: .continuous)
                .stroke(AppTheme.danger.opacity(0.2), lineWidth: 1)
        }
        .shadow(color: AppTheme.danger.opacity(0.12), radius: 22, x: 0, y: 12)
    }

    private func summaryPill(_ text: String) -> some View {
        Text(text)
            .font(.caption.weight(.bold))
            .foregroundStyle(AppTheme.muted)
            .padding(.horizontal, 10)
            .padding(.vertical, 6)
            .background(AppTheme.paper.opacity(0.7), in: Capsule())
            .overlay {
                Capsule()
                    .stroke(AppTheme.danger.opacity(0.14), lineWidth: 1)
            }
    }

    private func optionBackground(showCorrect: Bool, showWrong: Bool) -> Color {
        if showCorrect { return AppTheme.success.opacity(0.18) }
        if showWrong { return AppTheme.danger.opacity(0.12) }
        return AppTheme.paper.opacity(0.82)
    }

    private func optionBorder(showCorrect: Bool, showWrong: Bool) -> Color {
        if showCorrect { return AppTheme.successDark.opacity(0.42) }
        if showWrong { return AppTheme.danger.opacity(0.45) }
        return AppTheme.coral.opacity(0.18)
    }

    private func optionBadgeColor(showCorrect: Bool, showWrong: Bool) -> Color {
        if showCorrect { return AppTheme.success }
        if showWrong { return AppTheme.danger }
        return AppTheme.blush
    }

    private func startReviewSession() async {
        isStartingReview = true
        defer { isStartingReview = false }

        await sessionStore.loadDueCards()
        await sessionStore.loadReviewStats()
        guard !sessionStore.dueCards.isEmpty else {
            sessionStore.clearError()
            return
        }

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

        withAnimation(.easeOut(duration: 0.28)) {
            sessionDeck = deck
            sessionIndex = 0
            selectedOptionID = ""
            correctCount = 0
            wrongCount = 0
            pendingNextDue = ""
            sessionSummary = nil
            isAdvancingQuizCard = false
        }
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
    }

    private func advanceQuizCard() {
        guard !isAdvancingQuizCard else { return }

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

        withAnimation(.easeInOut(duration: 0.16)) {
            isAdvancingQuizCard = true
        }

        Task { @MainActor in
            try? await Task.sleep(nanoseconds: 130_000_000)
            sessionIndex = reviewed
            selectedOptionID = ""
            pendingNextDue = ""

            withAnimation(.spring(response: 0.34, dampingFraction: 0.86)) {
                isAdvancingQuizCard = false
            }
        }
    }

    private func endReviewSession() {
        withAnimation(.easeOut(duration: 0.24)) {
            sessionDeck = []
            sessionIndex = 0
            selectedOptionID = ""
            pendingNextDue = ""
            isAdvancingQuizCard = false
        }
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

private struct FocusedReviewContentHeightKey: PreferenceKey {
    static let defaultValue: CGFloat = 0

    static func reduce(value: inout CGFloat, nextValue: () -> CGFloat) {
        value = nextValue()
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
