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
                        await sessionStore.loadLibraryCards()
                    }
                }
            } else if filteredCards.isEmpty {
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
                        await sessionStore.loadLibraryCards()
                    }
                }
            } else {
                ZStack {
                    ReadingDeskBackground()
                    ScrollView {
                        LazyVStack(alignment: .leading, spacing: 14) {
                            filterPicker
                            statusMessages
                            ForEach(filteredCards) { card in
                                libraryRow(card)
                            }
                        }
                        .padding()
                    }
                    .refreshable {
                        await sessionStore.loadLibraryCards()
                    }
                }
            }
        }
        .navigationTitle("Library")
        .searchable(text: $searchText, prompt: "Search term or meaning")
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

    private var filteredCards: [DueCard] {
        let query = searchText.trimmingCharacters(in: .whitespacesAndNewlines).lowercased()
        return sessionStore.libraryCards.filter { card in
            selectedStatus.matches(card.state.status)
                && (query.isEmpty
                    || card.item.term.lowercased().contains(query)
                    || card.item.meaning.lowercased().contains(query)
                    || card.item.example_sentence.lowercased().contains(query)
                    || card.item.notes.lowercased().contains(query))
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

    private func libraryRow(_ card: DueCard) -> some View {
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
                Button("Edit") {
                    editingCard = card
                }
                .buttonStyle(.bordered)
                .disabled(sessionStore.isCreatingVocab || sessionStore.isDeletingVocab)

                Button("Delete", role: .destructive) {
                    deletingCard = card
                }
                .buttonStyle(.bordered)
                .disabled(sessionStore.isCreatingVocab || sessionStore.isDeletingVocab)
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
}
