import SwiftUI

struct LibraryView: View {
    @EnvironmentObject private var sessionStore: SessionStore
    @State private var editingCard: DueCard?
    @State private var deletingCard: DueCard?
    @State private var searchText = ""
    @State private var selectedStatus = LibraryStatusFilter.all

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
            } else if sessionStore.libraryCards.isEmpty {
                ZStack {
                    ReadingDeskBackground()
                    ScrollView {
                        VStack(alignment: .leading, spacing: 16) {
                            filterPicker
                            statusMessages
                            VStack(alignment: .leading, spacing: 12) {
                                Text("No matching cards")
                                    .readingTitle()
                                Text("Try a different search or status filter.")
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
                            filterPicker
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
        .onChange(of: selectedStatus) { _, _ in
            Task { await resetAndLoad() }
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

    private var filterPicker: some View {
        Picker("Status", selection: $selectedStatus) {
            ForEach(LibraryStatusFilter.allCases) { filter in
                Text(filter.title).tag(filter)
            }
        }
        .pickerStyle(.segmented)
        .tint(AppTheme.sage)
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
        await sessionStore.setLibraryPage(page, query: searchText, status: selectedStatus.queryValue)
    }

    private func loadCurrentPage() async {
        await sessionStore.loadLibraryCards(query: searchText, status: selectedStatus.queryValue)
    }

}

private struct LibraryCardRow: View {
    let card: DueCard
    let isBusy: Bool
    let onEdit: () -> Void
    let onDelete: () -> Void

    var body: some View {
        VStack(alignment: .leading, spacing: 10) {
            HStack(alignment: .firstTextBaseline) {
                Text(card.item.term)
                    .readingTerm()
                Spacer()
                Text(card.state.status)
                    .font(.caption.weight(.semibold))
                    .padding(.horizontal, 8)
                    .padding(.vertical, 4)
                    .foregroundStyle(AppTheme.sageDark)
                    .background(AppTheme.sage.opacity(0.12), in: Capsule())
            }

            Text(card.item.meaning.isEmpty ? "Meaning not added yet." : card.item.meaning)
                .foregroundStyle(.primary)

            if !card.item.example_sentence.isEmpty {
                Text(card.item.example_sentence)
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

            HStack {
                Button("Edit", action: onEdit)
                    .buttonStyle(.bordered)
                    .disabled(isBusy)

                Button("Delete", role: .destructive, action: onDelete)
                    .buttonStyle(.bordered)
                    .disabled(isBusy)
            }
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        .readingCard()
    }

    private func formattedDate(_ value: String) -> String {
        guard let date = ISO8601DateFormatter.vocabReview.date(from: value) else {
            return value
        }
        return date.formatted(date: .abbreviated, time: .shortened)
    }
}

private enum LibraryStatusFilter: String, CaseIterable, Identifiable {
    case all
    case new
    case learning
    case review

    var id: String { rawValue }

    var title: String {
        switch self {
        case .all:
            return "All"
        case .new:
            return "New"
        case .learning:
            return "Learning"
        case .review:
            return "Review"
        }
    }

    func matches(_ status: String) -> Bool {
        self == .all || rawValue == status
    }

    var queryValue: String {
        self == .all ? "" : rawValue
    }
}
