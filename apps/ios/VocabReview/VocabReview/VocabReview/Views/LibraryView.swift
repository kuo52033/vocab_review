import SwiftUI

struct LibraryView: View {
    @EnvironmentObject private var sessionStore: SessionStore
    @State private var editingCard: DueCard?
    @State private var deletingCard: DueCard?
    @State private var searchText = ""
    @State private var searchTask: Task<Void, Never>?
    @FocusState private var isSearchFocused: Bool

    private var searchQuery: String {
        searchText.trimmingCharacters(in: .whitespacesAndNewlines)
    }

    var body: some View {
        ZStack {
            ReadingDeskBackground()
            ScrollView {
                LazyVStack(alignment: .leading, spacing: 16) {
                    searchField
                    loadingIndicator

                    if sessionStore.libraryCards.isEmpty && !sessionStore.isLoadingLibraryCards {
                        emptyState
                    } else {
                        libraryResults
                    }
                }
                .padding()
                .animation(.easeOut(duration: 0.16), value: sessionStore.libraryCards.map(\.id))
                .animation(.easeOut(duration: 0.16), value: sessionStore.isLoadingLibraryCards)
            }
            .refreshable {
                await loadCurrentPage()
            }
        }
        .navigationTitle("")
        .onSubmit(of: .search) {
            searchTask?.cancel()
            Task { await resetAndLoad() }
        }
        .onChange(of: searchQuery.lowercased()) { _, _ in
            searchTask?.cancel()
            searchTask = Task {
                try? await Task.sleep(nanoseconds: 300_000_000)
                guard !Task.isCancelled else { return }
                await resetAndLoad()
            }
        }
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
            Text("This removes \"\(card.item.term)\" from your library and review queue.")
        }
        .onDisappear {
            searchTask?.cancel()
        }
    }

    private var searchField: some View {
        HStack(spacing: 10) {
            Image(systemName: "magnifyingglass")
                .foregroundStyle(AppTheme.muted)
            TextField(
                "",
                text: $searchText,
                prompt: Text("Search term or meaning")
                    .foregroundStyle(AppTheme.muted.opacity(0.82))
            )
                .font(.body.weight(.medium))
                .foregroundStyle(AppTheme.ink)
                .textInputAutocapitalization(.never)
                .autocorrectionDisabled()
                .submitLabel(.search)
                .tint(AppTheme.coral)
                .focused($isSearchFocused)
            if !searchText.isEmpty {
                Button {
                    searchText = ""
                    isSearchFocused = true
                } label: {
                    Image(systemName: "xmark.circle.fill")
                        .foregroundStyle(AppTheme.muted)
                }
                .buttonStyle(.plain)
                .accessibilityLabel("Clear search")
            }
        }
        .padding(.horizontal, 14)
        .padding(.vertical, 12)
        .background(AppTheme.paper, in: RoundedRectangle(cornerRadius: 16, style: .continuous))
        .overlay {
            RoundedRectangle(cornerRadius: 18, style: .continuous)
                .stroke(AppTheme.line, lineWidth: 1)
        }
        .shadow(color: AppTheme.shadow.opacity(0.08), radius: 14, x: 0, y: 6)
    }

    @ViewBuilder
    private var loadingIndicator: some View {
        if sessionStore.isLoadingLibraryCards {
            HStack(spacing: 10) {
                ProgressView()
                    .controlSize(.small)
                Text(searchQuery.isEmpty ? "Loading library..." : "Searching...")
                    .font(.caption.weight(.semibold))
                    .foregroundStyle(AppTheme.muted)
                Spacer()
            }
            .padding(.horizontal, 14)
            .padding(.vertical, 10)
            .background(AppTheme.paper.opacity(0.62), in: Capsule())
            .transition(.opacity)
        }
    }

    private var emptyState: some View {
        Text(searchQuery.isEmpty ? "No cards yet." : "No matching cards.")
            .font(AppTheme.displayFont(size: 38, weight: .semibold))
            .foregroundStyle(AppTheme.ink)
            .multilineTextAlignment(.center)
            .frame(maxWidth: .infinity, minHeight: 220)
        .transition(.opacity)
    }

    @ViewBuilder
    private var libraryResults: some View {
        ForEach(sessionStore.libraryCards) { card in
            LibraryCardRow(
                card: card,
                isBusy: sessionStore.isCreatingVocab || sessionStore.isDeletingVocab,
                isAudioPlaying: sessionStore.playingAudioVocabID == card.item.id,
                onEdit: { editingCard = card },
                onDelete: { deletingCard = card },
                onPlayAudio: {
                    Task { await sessionStore.toggleAudioPlayback(for: card.item) }
                }
            )
        }

        PaginationControl(
            page: sessionStore.libraryPage,
            hasNext: sessionStore.libraryHasNext,
            previous: {
                Task { await changePage(to: sessionStore.libraryPage - 1) }
            },
            next: {
                Task { await changePage(to: sessionStore.libraryPage + 1) }
            }
        )
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

    private func resetAndLoad() async {
        sessionStore.libraryPage = 1
        await loadCurrentPage()
    }

    private func changePage(to page: Int) async {
        await sessionStore.setLibraryPage(page, query: searchQuery)
    }

    private func loadCurrentPage() async {
        await sessionStore.loadLibraryCards(query: searchQuery)
    }

}

private struct LibraryCardRow: View {
    let card: DueCard
    let isBusy: Bool
    let isAudioPlaying: Bool
    let onEdit: () -> Void
    let onDelete: () -> Void
    let onPlayAudio: () -> Void
    @State private var isExpanded = false

    var body: some View {
        VStack(alignment: .leading, spacing: 0) {
            HStack(spacing: 10) {
                Button {
                    withAnimation(.spring(response: 0.32, dampingFraction: 0.9)) {
                        isExpanded.toggle()
                    }
                } label: {
                    HStack(spacing: 10) {
                        Image(systemName: "chevron.down")
                            .font(AppTheme.uiFont(size: 13, weight: .black, relativeTo: .caption))
                            .frame(width: 38, height: 38)
                            .background(isExpanded ? AppTheme.coral : AppTheme.blush.opacity(0.55), in: RoundedRectangle(cornerRadius: 12, style: .continuous))
                            .foregroundStyle(isExpanded ? AppTheme.paper : AppTheme.coral)
                            .rotationEffect(.degrees(isExpanded ? 180 : 0))
                        Text(card.item.term)
                            .font(AppTheme.displayFont(size: 22, weight: .semibold, relativeTo: .title3))
                            .foregroundStyle(AppTheme.ink)
                            .lineLimit(1)
                            .truncationMode(.tail)
                    }
                    .contentShape(Rectangle())
                }
                .buttonStyle(.plain)

                if card.item.hasPlayableAudio {
                    AudioPlayButton(isPlaying: isAudioPlaying, action: onPlayAudio)
                        .accessibilityLabel(isAudioPlaying ? "Pause pronunciation for \(card.item.term)" : "Play pronunciation for \(card.item.term)")
                }

                Spacer()

                Button(action: onEdit) {
                    Image(systemName: "pencil")
                        .font(.caption.weight(.semibold))
                        .frame(width: 28, height: 28)
                }
                .buttonStyle(LibraryRowActionButtonStyle())
                .disabled(isBusy)
                .accessibilityLabel("Edit \(card.item.term)")

                Button(role: .destructive, action: onDelete) {
                    Image(systemName: "xmark")
                        .font(.caption.weight(.semibold))
                        .frame(width: 28, height: 28)
                }
                .buttonStyle(LibraryRowActionButtonStyle())
                .disabled(isBusy)
                .accessibilityLabel("Delete \(card.item.term)")
            }
            .padding(.horizontal, 20)
            .padding(.vertical, 18)

            if isExpanded {
                details
                    .padding([.horizontal, .bottom])
                    .fixedSize(horizontal: false, vertical: true)
                    .transition(.blurFade)
            }
        }
        .animation(.spring(response: 0.32, dampingFraction: 0.9), value: isExpanded)
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(AppTheme.paper, in: RoundedRectangle(cornerRadius: 20, style: .continuous))
        .overlay {
            RoundedRectangle(cornerRadius: 20, style: .continuous)
                .stroke(isExpanded ? AppTheme.lineStrong : AppTheme.line, lineWidth: isExpanded ? 2 : 1)
        }
        .shadow(color: isExpanded ? AppTheme.coral.opacity(0.2) : AppTheme.shadow.opacity(0.08), radius: isExpanded ? 24 : 15, x: 0, y: isExpanded ? 12 : 4)
    }

    private var details: some View {
        VStack(alignment: .leading, spacing: 10) {
            if !card.item.part_of_speech.isEmpty {
                Text(card.item.part_of_speech.replacingOccurrences(of: "_", with: " "))
                    .font(.caption.weight(.semibold))
                    .foregroundStyle(AppTheme.sageDark)
                    .padding(.horizontal, 10)
                    .padding(.vertical, 5)
                    .background(AppTheme.sage.opacity(0.12), in: Capsule())
            }

            Text(card.item.meaning.isEmpty ? "Meaning not added yet." : card.item.meaning)
                .foregroundStyle(AppTheme.ink)
                .frame(maxWidth: .infinity, alignment: .leading)

            Text(card.item.chinese.isEmpty ? "Chinese not added yet." : card.item.chinese)
                .font(.callout.weight(.semibold))
                .foregroundStyle(AppTheme.clay)

            if !card.item.example_sentence.isEmpty {
                Text(card.item.example_sentence)
                    .font(.callout)
                    .italic()
                    .readingMuted()
                    .padding()
                    .frame(maxWidth: .infinity, alignment: .leading)
                    .background(AppTheme.blush.opacity(0.58), in: RoundedRectangle(cornerRadius: 12, style: .continuous))
                    .overlay(alignment: .leading) {
                        RoundedRectangle(cornerRadius: 2)
                            .fill(AppTheme.coral)
                            .frame(width: 3)
                            .padding(.vertical, 10)
                    }
            }

            if !card.item.notes.isEmpty {
                Text("Notes: \(card.item.notes)")
                    .font(.callout)
                    .readingMuted()
            }

            Text("Next due: \(formattedDate(card.state.next_due_at))")
                .font(.caption)
                .readingMuted()
        }
    }

    private func formattedDate(_ value: String) -> String {
        guard let date = ISO8601DateFormatter.vocabReview.date(from: value) else {
            return value
        }
        return date.formatted(date: .abbreviated, time: .shortened)
    }
}

private struct LibraryRowActionButtonStyle: ButtonStyle {
    @Environment(\.isEnabled) private var isEnabled

    func makeBody(configuration: Configuration) -> some View {
        configuration.label
            .foregroundStyle(isEnabled ? AppTheme.coral : AppTheme.muted.opacity(0.45))
            .background(
                AppTheme.rose100.opacity(configuration.isPressed ? 0.5 : 0.32),
                in: Circle()
            )
            .opacity(isEnabled ? 1 : 0.55)
            .animation(.easeOut(duration: 0.12), value: configuration.isPressed)
            .animation(.easeOut(duration: 0.12), value: isEnabled)
    }
}

private struct BlurFadeModifier: ViewModifier {
    let opacity: Double
    let blurRadius: CGFloat

    func body(content: Content) -> some View {
        content
            .opacity(opacity)
            .blur(radius: blurRadius)
    }
}

private extension AnyTransition {
    static var blurFade: AnyTransition {
        .modifier(
            active: BlurFadeModifier(opacity: 0, blurRadius: 8),
            identity: BlurFadeModifier(opacity: 1, blurRadius: 0)
        )
    }
}
