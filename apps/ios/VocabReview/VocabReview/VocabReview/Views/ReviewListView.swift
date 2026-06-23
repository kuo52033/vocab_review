import SwiftUI

struct ReviewListView: View {
    @EnvironmentObject private var sessionStore: SessionStore
    @Binding private var isReviewSessionActive: Bool
    @State private var sessionDeck: [QuizCard] = []
    @State private var sessionIndex = 0
    @State private var selectedOptionID = ""
    @State private var correctCount = 0
    @State private var wrongCount = 0
    @State private var wrongReviewItems: [WrongReviewItem] = []
    @State private var pendingNextDue = ""
    @State private var sessionSummary: ReviewSessionSummary?
    @State private var isStartingReview = false
    @State private var isAdvancingQuizCard = false
    @State private var isReviewHomePresented = false
    @State private var focusedReviewContentHeight: CGFloat = 0

    private let sessionLimit = 12
    private let focusedReviewTopPadding: CGFloat = 18
    private let focusedReviewBottomPadding: CGFloat = 18
    private let focusedReviewActionInsetHeight: CGFloat = 72

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
            } else if let summary = sessionSummary {
                finalReviewPage(summary)
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
            VStack(alignment: .leading, spacing: 24) {
                startReviewCard
                    .opacity(isReviewHomePresented ? 1 : 0)
                    .scaleEffect(isReviewHomePresented ? 1 : 0.985)
                    .offset(y: isReviewHomePresented ? 0 : 24)
                    .transition(.opacity.combined(with: .scale(scale: 0.985)))

                homeStats
            }
            .padding()
        }
        .refreshable {
            await sessionStore.loadDueCards()
            await sessionStore.loadReviewStats()
        }
        .onAppear {
            isReviewHomePresented = false
            withAnimation(.spring(response: 0.88, dampingFraction: 0.82).delay(0.12)) {
                isReviewHomePresented = true
            }
        }
    }

    private func focusedReviewSession(_ card: QuizCard) -> some View {
        GeometryReader { proxy in
            let actionInsetHeight = selectedOptionID.isEmpty ? 0 : focusedReviewActionInsetHeight
            let availableHeight = max(proxy.size.height - actionInsetHeight, 0)
            let bottomPadding = max(focusedReviewBottomPadding, proxy.safeAreaInsets.bottom + focusedReviewBottomPadding)
            let hasOversizedOption = card.options.contains { option in
                option.text.contains("\n") || option.text.count > 180
            }
            let shouldScroll = focusedReviewContentHeight > availableHeight + 1 || hasOversizedOption

            ScrollViewReader { scrollProxy in
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
                    .padding(.top, focusedReviewTopPadding)
                    .padding(.bottom, bottomPadding)
                    .background {
                        GeometryReader { contentProxy in
                            Color.clear
                                .preference(key: FocusedReviewContentHeightKey.self, value: contentProxy.size.height)
                        }
                    }
                    .frame(
                        maxWidth: .infinity,
                        minHeight: availableHeight,
                        alignment: shouldScroll ? .top : .center
                    )
                }
                .scrollIndicators(.hidden)
                .scrollBounceBehavior(.basedOnSize, axes: .vertical)
                .onPreferenceChange(FocusedReviewContentHeightKey.self) { focusedReviewContentHeight = $0 }
                .onChange(of: card.card.item.id) { _, newID in
                    focusedReviewContentHeight = 0
                    DispatchQueue.main.async {
                        scrollProxy.scrollTo(newID, anchor: .top)
                    }
                }
            }
            .safeAreaInset(edge: .bottom, spacing: 0) {
                if !selectedOptionID.isEmpty {
                    nextReviewAction
                }
            }
            .refreshable {
                await sessionStore.loadDueCards()
                await sessionStore.loadReviewStats()
            }
        }
    }

    private var startReviewCard: some View {
        VStack(alignment: .leading, spacing: 28) {
            HStack {
                Text("Quiz Mode")
                    .font(.caption.weight(.bold))
                    .textCase(.uppercase)
                    .foregroundStyle(AppTheme.paper.opacity(0.74))
                Spacer()
                Text("\(sessionStore.reviewStats.due_now) due now")
                    .font(.caption.weight(.semibold))
                    .foregroundStyle(AppTheme.paper)
                    .padding(.horizontal, 14)
                    .padding(.vertical, 8)
                    .background(AppTheme.paper.opacity(0.2), in: Capsule())
            }

            VStack(alignment: .leading, spacing: 14) {
                Text(sessionStore.reviewStats.due_now == 0 ? "Clear desk." : "Ready when you are.")
                    .font(AppTheme.displayFont(size: 44, weight: .semibold))
                    .foregroundStyle(AppTheme.paper)
                    .lineSpacing(6)
                Text(sessionStore.reviewStats.due_now == 0 ? "No due cards right now." : "A short multiple-choice sprint is waiting.")
                    .foregroundStyle(AppTheme.paper.opacity(0.78))
            }

            if sessionStore.reviewStats.due_now > 0 {
                Button {
                    Task { await startReviewSession() }
                } label: {
                    if isStartingReview {
                        ProgressView()
                            .frame(maxWidth: .infinity)
                    } else {
                        Text("Start Review ->")
                            .frame(maxWidth: .infinity)
                    }
                }
                .buttonStyle(ReviewHeroButtonStyle())
                .disabled(sessionStore.dueCards.isEmpty || isStartingReview)
            }
        }
        .padding(28)
        .frame(minHeight: 344)
        .frame(maxWidth: .infinity, alignment: .leading)
        .background {
            RoundedRectangle(cornerRadius: 28, style: .continuous)
                .fill(
                    LinearGradient(
                        colors: [AppTheme.reviewGradientStart, AppTheme.reviewGradientEnd],
                        startPoint: .topLeading,
                        endPoint: .bottomTrailing
                    )
                )
                .overlay(alignment: .topTrailing) {
                    Circle()
                        .fill(AppTheme.paper.opacity(0.08))
                        .frame(width: 210, height: 210)
                        .offset(x: 74, y: -70)
                }
                .overlay(alignment: .bottomLeading) {
                    Circle()
                        .fill(AppTheme.paper.opacity(0.08))
                        .frame(width: 180, height: 180)
                        .offset(x: -72, y: 72)
                }
        }
        .overlay {
            RoundedRectangle(cornerRadius: 28, style: .continuous)
                .stroke(AppTheme.paper.opacity(0.12), lineWidth: 1)
        }
        .shadow(color: AppTheme.reviewGradientStart.opacity(0.38), radius: 34, x: 0, y: 24)
    }

    private var homeStats: some View {
        HStack(spacing: 14) {
            reviewStatTile("flame.fill", "\(max(sessionStore.reviewStats.reviewed_today, 0))", "Day streak", .orange)
            reviewStatTile("books.vertical.fill", "\(sessionStore.reviewStats.due_now)", "Due today", AppTheme.coral)
            reviewStatTile("checkmark.square.fill", "\(sessionStore.reviewStats.active_cards)", "Mastered", AppTheme.success)
        }
    }

    private func reviewStatTile(_ systemImage: String, _ value: String, _ label: String, _ iconColor: Color) -> some View {
        VStack(spacing: 8) {
            Image(systemName: systemImage)
                .font(.title3.weight(.bold))
                .foregroundStyle(iconColor)
            Text(value)
                .font(AppTheme.uiFont(size: 32, weight: .black, relativeTo: .title))
                .foregroundStyle(AppTheme.ink)
            Text(label)
                .font(.caption.weight(.bold))
                .foregroundStyle(AppTheme.muted)
                .lineLimit(1)
                .minimumScaleFactor(0.8)
        }
        .frame(maxWidth: .infinity)
        .frame(height: 136)
        .background(AppTheme.paper, in: RoundedRectangle(cornerRadius: 20, style: .continuous))
        .overlay {
            RoundedRectangle(cornerRadius: 20, style: .continuous)
                .stroke(AppTheme.line, lineWidth: 1)
        }
        .shadow(color: AppTheme.shadow.opacity(0.08), radius: 15, x: 0, y: 4)
    }

    private func finalReviewPage(_ summary: ReviewSessionSummary) -> some View {
        GeometryReader { proxy in
            ScrollView {
                VStack(spacing: 34) {
                    Spacer(minLength: max(proxy.size.height * 0.12, 72))

                    ReviewAccuracyRing(summary: summary)

                    Text(summary.resultMessage)
                        .font(AppTheme.displayFont(size: 42, weight: .semibold))
                        .foregroundStyle(AppTheme.ink)
                        .multilineTextAlignment(.center)

                    if !summary.wrongReviews.isEmpty {
                        wrongReviewSection(summary.wrongReviews)
                    }

                    Button {
                        withAnimation(.spring(response: 0.34, dampingFraction: 0.86)) {
                            sessionSummary = nil
                        }
                    } label: {
                        Text("Back to Home")
                            .frame(maxWidth: .infinity)
                    }
                    .buttonStyle(ReviewResultButtonStyle())
                    .frame(maxWidth: 300)

                    Spacer(minLength: 120)
                }
                .frame(maxWidth: .infinity)
                .frame(minHeight: proxy.size.height)
                .padding(.horizontal, 24)
            }
            .scrollIndicators(.hidden)
        }
    }

    private func wrongReviewSection(_ items: [WrongReviewItem]) -> some View {
        VStack(alignment: .center, spacing: 14) {
            Text("Review these again")
                .font(AppTheme.displayFont(size: 24, weight: .semibold, relativeTo: .title2))
                .foregroundStyle(AppTheme.ink)
                .multilineTextAlignment(.center)
                .frame(maxWidth: .infinity)

            if items.count == 1, let item = items.first {
                wrongReviewCard(item)
                    .frame(maxWidth: .infinity)
            } else {
                ScrollView(.horizontal) {
                    LazyHStack(alignment: .top, spacing: 12) {
                        ForEach(items) { item in
                            wrongReviewCard(item)
                                .containerRelativeFrame(.horizontal, count: 1, spacing: 12)
                        }
                    }
                    .scrollTargetLayout()
                }
                .scrollIndicators(.hidden)
                .scrollTargetBehavior(.viewAligned)
            }
        }
        .frame(maxWidth: 560, alignment: .center)
    }

    private func wrongReviewCard(_ item: WrongReviewItem) -> some View {
        VStack(alignment: .leading, spacing: 12) {
            VStack(alignment: .leading, spacing: 4) {
                Text(item.term)
                    .font(AppTheme.displayFont(size: 34, weight: .semibold))
                    .foregroundStyle(AppTheme.ink)
                    .lineLimit(nil)
                    .fixedSize(horizontal: false, vertical: true)
                if !item.chinese.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty {
                    Text(item.chinese)
                        .font(.callout.weight(.bold))
                        .foregroundStyle(AppTheme.coral)
                }
            }

            wrongReviewField("English explanation", item.meaning)
            wrongReviewChoiceField(item)
        }
        .padding()
        .frame(minHeight: 230, alignment: .topLeading)
        .background(AppTheme.paper.opacity(0.9), in: RoundedRectangle(cornerRadius: 18, style: .continuous))
        .overlay {
            RoundedRectangle(cornerRadius: 18, style: .continuous)
                .stroke(AppTheme.danger.opacity(0.18), lineWidth: 1)
        }
        .shadow(color: AppTheme.shadow.opacity(0.08), radius: 14, x: 0, y: 8)
    }

    private func wrongReviewField(_ label: String, _ value: String) -> some View {
        VStack(alignment: .leading, spacing: 4) {
            Text(label)
                .font(.caption.weight(.bold))
                .textCase(.uppercase)
                .foregroundStyle(AppTheme.muted)
            Text(value)
                .font(.callout.weight(.semibold))
                .foregroundStyle(AppTheme.ink)
                .lineLimit(nil)
                .fixedSize(horizontal: false, vertical: true)
        }
    }

    private func wrongReviewChoiceField(_ item: WrongReviewItem) -> some View {
        VStack(alignment: .leading, spacing: 8) {
            Text("You chose")
                .font(.caption.weight(.bold))
                .textCase(.uppercase)
                .foregroundStyle(AppTheme.muted)

            Text("\(item.selectedOptionLabel). \(item.selectedAnswer)")
                .font(.callout.weight(.semibold))
                .foregroundStyle(AppTheme.ink)
                .lineLimit(nil)
                .fixedSize(horizontal: false, vertical: true)

            let source = selectedSourceLabel(for: item)
            if !source.isEmpty {
                Text(source)
                    .font(.callout.weight(.bold))
                    .foregroundStyle(AppTheme.coral)
                    .lineLimit(nil)
                    .fixedSize(horizontal: false, vertical: true)
                    .padding(.top, 2)
                    .padding(.horizontal, 10)
                    .padding(.vertical, 7)
                    .background(AppTheme.blush.opacity(0.58), in: RoundedRectangle(cornerRadius: 12, style: .continuous))
                    .overlay {
                        RoundedRectangle(cornerRadius: 12, style: .continuous)
                            .stroke(AppTheme.coral.opacity(0.14), lineWidth: 1)
                    }
            }
        }
    }

    private func selectedSourceLabel(for item: WrongReviewItem) -> String {
        let selectedChinese = item.selectedChinese.trimmingCharacters(in: .whitespacesAndNewlines)
        let selectedTerm = item.selectedTerm.trimmingCharacters(in: .whitespacesAndNewlines)
        if selectedTerm.isEmpty { return "" }
        if selectedChinese.isEmpty { return selectedTerm }
        return "\(selectedTerm) · \(selectedChinese)"
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
                HStack(alignment: .center, spacing: 10) {
                    Text(quizCard.card.item.term)
                        .font(AppTheme.displayFont(size: 52, weight: .semibold))
                        .foregroundStyle(AppTheme.ink)
                        .multilineTextAlignment(.center)
                        .tracking(0)
                        .lineLimit(2)
                        .minimumScaleFactor(0.72)
                        .layoutPriority(1)
                    if quizCard.card.item.hasPlayableAudio {
                        AudioPlayButton(
                            isPlaying: sessionStore.playingAudioVocabID == quizCard.card.item.id,
                            action: {
                                Task { await sessionStore.toggleAudioPlayback(for: quizCard.card.item) }
                            }
                        )
                        .accessibilityLabel(
                            sessionStore.playingAudioVocabID == quizCard.card.item.id
                                ? "Pause pronunciation for \(quizCard.card.item.term)"
                                : "Play pronunciation for \(quizCard.card.item.term)"
                        )
                    }
                }
                .fixedSize(horizontal: false, vertical: true)
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
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        .readingCard()
    }

    private var nextReviewAction: some View {
        Button {
            advanceQuizCard()
        } label: {
            Text(sessionIndex + 1 >= sessionDeck.count ? "Show summary" : "Next word")
                .frame(maxWidth: .infinity)
        }
        .readingPrimaryButton()
        .disabled(sessionStore.isGrading || isAdvancingQuizCard)
        .padding(.horizontal, 18)
        .padding(.top, 8)
        .padding(.bottom, 8)
        .background(AppTheme.linen.opacity(0.96))
    }

    private func answerOptionButton(_ option: QuizOption, index: Int) -> some View {
        let isSelected = selectedOptionID == option.id
        let showCorrect = !selectedOptionID.isEmpty && option.isCorrect
        let showWrong = isSelected && !option.isCorrect
        let optionLabel = String(UnicodeScalar(65 + index)!)

        return Button {
            guard selectedOptionID.isEmpty, !sessionStore.isGrading else { return }
            Task { await selectAnswer(option, optionLabel: optionLabel) }
        } label: {
            HStack(alignment: .top, spacing: 12) {
                Text(optionLabel)
                    .font(.caption.weight(.bold))
                    .foregroundStyle(showCorrect ? AppTheme.paper : AppTheme.ink)
                    .frame(width: 28, height: 28)
                    .background {
                        if showCorrect {
                            Circle()
                                .fill(
                                    LinearGradient(
                                        colors: [AppTheme.success, AppTheme.successDark],
                                        startPoint: .topLeading,
                                        endPoint: .bottomTrailing
                                    )
                                )
                        } else {
                            Circle()
                                .fill(optionBadgeColor(showWrong: showWrong))
                        }
                    }
                VStack(alignment: .leading, spacing: 6) {
                    Text(option.text)
                        .multilineTextAlignment(.leading)
                        .lineLimit(nil)
                        .foregroundStyle(AppTheme.ink)
                    if showWrong {
                        Text(wrongOptionSourceLabel(option.item))
                            .font(.callout.weight(.bold))
                            .foregroundStyle(AppTheme.coral)
                            .lineLimit(nil)
                            .fixedSize(horizontal: false, vertical: true)
                    }
                }
                .frame(maxWidth: .infinity, alignment: .leading)
                .fixedSize(horizontal: false, vertical: true)
                .layoutPriority(1)
            }
            .fixedSize(horizontal: false, vertical: true)
            .padding()
            .frame(maxWidth: .infinity, alignment: .leading)
        }
        .buttonStyle(.plain)
        .disabled(!selectedOptionID.isEmpty || sessionStore.isGrading)
        .contentShape(RoundedRectangle(cornerRadius: 18, style: .continuous))
        .background {
            let shape = RoundedRectangle(cornerRadius: 18, style: .continuous)
            if showCorrect {
                shape.fill(
                    LinearGradient(
                        colors: [AppTheme.successWash.opacity(0.98), AppTheme.successPale.opacity(0.88)],
                        startPoint: .topLeading,
                        endPoint: .bottomTrailing
                    )
                )
            } else {
                shape.fill(optionBackground(showWrong: showWrong))
            }
        }
        .overlay {
            RoundedRectangle(cornerRadius: 18, style: .continuous)
                .stroke(optionBorder(showCorrect: showCorrect, showWrong: showWrong), lineWidth: 1)
        }
        .accessibilityElement(children: .combine)
        .accessibilityAddTraits(.isButton)
        .accessibilityLabel("Option \(optionLabel), \(option.text)")
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
                        .lineLimit(nil)
                        .fixedSize(horizontal: false, vertical: true)
                        .readingMuted()
                }
                if !item.example_sentence.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty {
                    Text(item.example_sentence)
                        .font(.footnote)
                        .lineLimit(nil)
                        .fixedSize(horizontal: false, vertical: true)
                        .readingMuted()
                }
            }
            .frame(maxWidth: .infinity, alignment: .leading)
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

    private func wrongOptionSourceLabel(_ item: VocabItem) -> String {
        let chinese = item.chinese.trimmingCharacters(in: .whitespacesAndNewlines)
        if chinese.isEmpty {
            return item.term
        }
        return "\(item.term) · \(chinese)"
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
                        .font(AppTheme.uiFont(size: 32, weight: .bold, relativeTo: .title))
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

    private func optionBackground(showWrong: Bool) -> Color {
        if showWrong { return AppTheme.danger.opacity(0.12) }
        return AppTheme.paper.opacity(0.82)
    }

    private func optionBorder(showCorrect: Bool, showWrong: Bool) -> Color {
        if showCorrect { return AppTheme.successDark.opacity(0.62) }
        if showWrong { return AppTheme.danger.opacity(0.45) }
        return AppTheme.coral.opacity(0.18)
    }

    private func optionBadgeColor(showWrong: Bool) -> Color {
        if showWrong { return AppTheme.danger }
        return AppTheme.blush
    }

    private func startReviewSession() async {
        isStartingReview = true
        defer { isStartingReview = false }

        guard let bootstrap = await sessionStore.loadReviewBootstrap(limit: 100) else {
            return
        }

        guard !bootstrap.due.isEmpty else {
            sessionStore.clearError()
            return
        }

        let deck = buildQuizDeck(
            dueCards: bootstrap.due,
            candidates: bootstrap.candidates,
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
            wrongReviewItems = []
            pendingNextDue = ""
            sessionSummary = nil
            isAdvancingQuizCard = false
        }
        sessionStore.clearError()
    }

    private func selectAnswer(_ option: QuizOption, optionLabel: String) async {
        guard let currentQuizCard, selectedOptionID.isEmpty else { return }
        selectedOptionID = option.id
        let grade = option.isCorrect ? "easy" : "again"
        guard let nextDue = await sessionStore.gradeAndReturnNextDue(cardID: currentQuizCard.card.item.id, grade: grade) else {
            selectedOptionID = ""
            return
        }
        correctCount += option.isCorrect ? 1 : 0
        wrongCount += option.isCorrect ? 0 : 1
        if !option.isCorrect {
            wrongReviewItems.append(
                WrongReviewItem(
                    id: "\(currentQuizCard.card.item.id)-\(sessionIndex)",
                    term: currentQuizCard.card.item.term,
                    meaning: currentQuizCard.card.item.meaning,
                    chinese: currentQuizCard.card.item.chinese,
                    selectedAnswer: option.text,
                    selectedOptionLabel: optionLabel,
                    selectedTerm: option.item.term,
                    selectedChinese: option.item.chinese
                )
            )
        }
        pendingNextDue = nextDue
    }

    private func advanceQuizCard() {
        guard !isAdvancingQuizCard else { return }
        sessionStore.stopAudioPlayback()

        let reviewed = sessionIndex + 1
        if reviewed >= sessionDeck.count {
            sessionSummary = ReviewSessionSummary(
                reviewed: reviewed,
                correct: correctCount,
                wrong: wrongCount,
                lastNextDue: pendingNextDue.isEmpty ? nil : pendingNextDue,
                wrongReviews: wrongReviewItems
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
        sessionStore.stopAudioPlayback()
        withAnimation(.easeOut(duration: 0.24)) {
            sessionDeck = []
            sessionIndex = 0
            selectedOptionID = ""
            pendingNextDue = ""
            wrongReviewItems = []
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
    let item: VocabItem
}

private struct ReviewSessionSummary {
    let reviewed: Int
    let correct: Int
    let wrong: Int
    let lastNextDue: String?
    let wrongReviews: [WrongReviewItem]

    var accuracy: Int {
        Int((Double(correct) / Double(max(reviewed, 1)) * 100).rounded())
    }

    var accuracyFraction: Double {
        Double(correct) / Double(max(reviewed, 1))
    }

    var resultMessage: String {
        switch accuracy {
        case 100:
            return "Perfect! ✨"
        case 80...99:
            return "Great work! 🎉"
        case 50...79:
            return "Nice progress! 👍"
        default:
            return "Keep going! 💪"
        }
    }
}

private struct WrongReviewItem: Identifiable {
    let id: String
    let term: String
    let meaning: String
    let chinese: String
    let selectedAnswer: String
    let selectedOptionLabel: String
    let selectedTerm: String
    let selectedChinese: String
}

private struct FocusedReviewContentHeightKey: PreferenceKey {
    static let defaultValue: CGFloat = 0

    static func reduce(value: inout CGFloat, nextValue: () -> CGFloat) {
        value = nextValue()
    }
}

private struct ReviewHeroButtonStyle: ButtonStyle {
    @Environment(\.isEnabled) private var isEnabled

    func makeBody(configuration: Configuration) -> some View {
        configuration.label
            .font(AppTheme.uiFont(size: 17, weight: .black))
            .foregroundStyle(isEnabled ? AppTheme.coral : AppTheme.muted.opacity(0.65))
            .padding(.vertical, 18)
            .background(AppTheme.paper.opacity(configuration.isPressed ? 0.9 : 1.0), in: RoundedRectangle(cornerRadius: 16, style: .continuous))
            .shadow(color: Color.black.opacity(isEnabled ? 0.15 : 0), radius: 18, x: 0, y: 10)
            .scaleEffect(configuration.isPressed ? 0.98 : 1)
            .animation(.easeOut(duration: 0.14), value: configuration.isPressed)
            .animation(.easeOut(duration: 0.14), value: isEnabled)
    }
}

private struct ReviewResultButtonStyle: ButtonStyle {
    @Environment(\.isEnabled) private var isEnabled

    func makeBody(configuration: Configuration) -> some View {
        configuration.label
            .font(AppTheme.uiFont(size: 20, weight: .black, relativeTo: .title3))
            .foregroundStyle(isEnabled ? AppTheme.paper : AppTheme.muted.opacity(0.65))
            .padding(.vertical, 22)
            .background(AppTheme.coral.opacity(configuration.isPressed ? 0.86 : 1.0), in: RoundedRectangle(cornerRadius: 22, style: .continuous))
            .shadow(color: AppTheme.shadow.opacity(isEnabled ? 0.16 : 0), radius: 18, x: 0, y: 10)
            .scaleEffect(configuration.isPressed ? 0.98 : 1)
            .animation(.easeOut(duration: 0.14), value: configuration.isPressed)
            .animation(.easeOut(duration: 0.14), value: isEnabled)
    }
}

private struct ReviewAccuracyRing: View {
    let summary: ReviewSessionSummary
    @State private var displayedFraction = 0.0
    private let ringSize: CGFloat = 156
    private let ringLineWidth: CGFloat = 20

    var body: some View {
        ZStack {
            Circle()
                .stroke(AppTheme.line.opacity(0.45), lineWidth: 3)
                .frame(width: 176, height: 176)

            Circle()
                .stroke(AppTheme.paper.opacity(0.42), lineWidth: ringLineWidth)
                .frame(width: ringSize, height: ringSize)

            Circle()
                .trim(from: 0, to: displayedFraction)
                .stroke(
                    AppTheme.coral,
                    style: StrokeStyle(lineWidth: ringLineWidth, lineCap: .round)
                )
                .frame(width: ringSize, height: ringSize)
                .rotationEffect(.degrees(-90))

            VStack(spacing: 6) {
                Text("\(summary.correct)/\(summary.reviewed)")
                    .font(AppTheme.uiFont(size: 44, weight: .black, relativeTo: .largeTitle))
                    .foregroundStyle(AppTheme.coral)
                    .lineLimit(1)
                    .minimumScaleFactor(0.62)
                    .allowsTightening(true)
                Text("correct")
                    .font(.callout.weight(.black))
                    .foregroundStyle(AppTheme.muted)
            }
            .frame(width: 118)
        }
        .frame(width: 190, height: 190)
        .shadow(color: AppTheme.shadow.opacity(0.08), radius: 24, x: 0, y: 12)
        .onAppear {
            displayedFraction = 0
            withAnimation(.easeOut(duration: 0.72).delay(0.12)) {
                displayedFraction = summary.accuracyFraction
            }
        }
        .onChange(of: summary.accuracyFraction) { _, newValue in
            withAnimation(.easeOut(duration: 0.72)) {
                displayedFraction = newValue
            }
        }
    }
}

private func buildQuizDeck(dueCards: [DueCard], candidates: [DueCard], limit: Int) -> [QuizCard] {
    let cardsWithAnswers = dueCards.filter { !$0.item.meaning.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty }
    let candidateAnswers = candidates
        .filter { !$0.item.meaning.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty }
        .map { (id: $0.item.id, text: $0.item.meaning.trimmingCharacters(in: .whitespacesAndNewlines), item: $0.item) }

    return cardsWithAnswers.shuffled()
        .prefix(limit)
        .compactMap { card in
            let correctText = card.item.meaning.trimmingCharacters(in: .whitespacesAndNewlines)
            let distractors = candidateAnswers
                .filter { $0.id != card.item.id && $0.text != correctText }
                .shuffled()
                .prefix(3)

            let options = ([QuizOption(id: "\(card.item.id)-correct", text: correctText, isCorrect: true, item: card.item)] + distractors.map {
                QuizOption(id: "\(card.item.id)-\($0.id)", text: $0.text, isCorrect: false, item: $0.item)
            }).shuffled()

            guard options.count >= 2 else { return nil }
            return QuizCard(card: card, options: options)
        }
}
