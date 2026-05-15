import SwiftUI

struct LibraryView: View {
    @EnvironmentObject private var sessionStore: SessionStore
    @State private var editingCard: DueCard?
    @State private var deletingCard: DueCard?
    @State private var searchText = ""

    var body: some View {
        Group {
            if sessionStore.isLoadingLibraryCards && sessionStore.libraryCards.isEmpty {
                ZStack {
                    ReadingDeskBackground()
                    ProgressView("Loading library...")
                        .padding()
                        .readingCard()
                }
            } else if sessionStore.libraryCards.isEmpty {
                ZStack {
                    ReadingDeskBackground()
                    ScrollView {
                        VStack(alignment: .leading, spacing: 16) {
                            statusMessages
                            VStack(alignment: .leading, spacing: 12) {
                                Text("No cards yet")
                                    .readingTitle()
                                Text("Use Add to create your first vocabulary card.")
                                    .readingMuted()
                            }
                            .readingCard()
                        }
                        .padding()
                    }
                    .refreshable {
                        await loadCurrentPage()
                    }
                }
            } else {
                ZStack {
                    ReadingDeskBackground()
                    ScrollView {
                        LazyVStack(alignment: .leading, spacing: 14) {
                            statusMessages
                            ForEach(sessionStore.libraryCards) { card in
                                LibraryCardRow(
                                    card: card,
                                    isBusy: sessionStore.isCreatingVocab || sessionStore.isDeletingVocab,
                                    onEdit: { editingCard = card },
                                    onDelete: { deletingCard = card }
                                )
                            }
                            PaginationControl(
                                page: sessionStore.libraryPage,
                                totalPages: sessionStore.libraryPageCount,
                                previous: {
                                    Task { await changePage(to: sessionStore.libraryPage - 1) }
                                },
                                next: {
                                    Task { await changePage(to: sessionStore.libraryPage + 1) }
                                }
                            )
                        }
                        .padding()
                    }
                    .refreshable {
                        await loadCurrentPage()
                    }
                }
            }
        }
        .navigationTitle("Library")
        .searchable(text: $searchText, prompt: "Search term or meaning")
        .onSubmit(of: .search) {
            Task { await resetAndLoad() }
        }
        .onChange(of: searchText) { _, newValue in
            if newValue.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty {
                Task { await resetAndLoad() }
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
        if !sessionStore.infoMessage.isEmpty {
            Text(sessionStore.infoMessage)
                .foregroundStyle(.secondary)
        }
    }

    private func resetAndLoad() async {
        sessionStore.libraryPage = 1
        await loadCurrentPage()
    }

    private func changePage(to page: Int) async {
        await sessionStore.setLibraryPage(page, query: searchText)
    }

    private func loadCurrentPage() async {
        await sessionStore.loadLibraryCards(query: searchText)
    }

}

private struct LibraryCardRow: View {
    let card: DueCard
    let isBusy: Bool
    let onEdit: () -> Void
    let onDelete: () -> Void
    @State private var isExpanded = false

    var body: some View {
        VStack(alignment: .leading, spacing: 0) {
            HStack(spacing: 12) {
                Button {
                    withAnimation(.easeInOut(duration: 0.2)) {
                        isExpanded.toggle()
                    }
                } label: {
                    HStack(spacing: 10) {
                        Image(systemName: isExpanded ? "chevron.down" : "chevron.right")
                            .font(.caption.weight(.bold))
                            .foregroundStyle(AppTheme.muted)
                        Text(card.item.term)
                            .font(.system(.title3, design: .default, weight: .medium))
                            .foregroundStyle(AppTheme.sageDark)
                        Spacer()
                    }
                    .contentShape(Rectangle())
                }
                .buttonStyle(.plain)

                Button(action: onEdit) {
                    Image(systemName: "pencil")
                        .frame(width: 34, height: 34)
                }
                .buttonStyle(.bordered)
                .disabled(isBusy)
                .accessibilityLabel("Edit \(card.item.term)")

                Button(role: .destructive, action: onDelete) {
                    Image(systemName: "xmark")
                        .frame(width: 34, height: 34)
                }
                .buttonStyle(.bordered)
                .disabled(isBusy)
                .accessibilityLabel("Delete \(card.item.term)")
            }
            .padding()

            if isExpanded {
                VStack(alignment: .leading, spacing: 10) {
                    HStack(alignment: .firstTextBaseline) {
                        Text(card.item.meaning.isEmpty ? "Meaning not added yet." : card.item.meaning)
                            .foregroundStyle(AppTheme.ink)
                        Spacer()
                        if !card.item.part_of_speech.isEmpty {
                            Text(card.item.part_of_speech.replacingOccurrences(of: "_", with: " "))
                                .font(.caption.weight(.semibold))
                                .foregroundStyle(AppTheme.sageDark)
                                .padding(.horizontal, 10)
                                .padding(.vertical, 5)
                                .background(AppTheme.sage.opacity(0.12), in: Capsule())
                        }
                    }

                    if !card.item.example_sentence.isEmpty {
                        Text(card.item.example_sentence)
                            .font(.callout)
                            .readingMuted()
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
                .padding([.horizontal, .bottom])
                .transition(.opacity.combined(with: .move(edge: .top)))
            }
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(AppTheme.paper.opacity(0.88), in: RoundedRectangle(cornerRadius: 22, style: .continuous))
        .overlay {
            RoundedRectangle(cornerRadius: 22, style: .continuous)
                .stroke(AppTheme.ink.opacity(0.08), lineWidth: 1)
        }
        .shadow(color: AppTheme.ink.opacity(0.06), radius: 14, x: 0, y: 8)
    }

    private func formattedDate(_ value: String) -> String {
        guard let date = ISO8601DateFormatter.vocabReview.date(from: value) else {
            return value
        }
        return date.formatted(date: .abbreviated, time: .shortened)
    }
}
