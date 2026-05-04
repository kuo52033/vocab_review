import SwiftUI

struct LibraryView: View {
    @EnvironmentObject private var sessionStore: SessionStore
    @State private var editingCard: DueCard?
    @State private var deletingCard: DueCard?

    var body: some View {
        Group {
            if sessionStore.isLoadingLibraryCards && sessionStore.libraryCards.isEmpty {
                ProgressView("Loading library...")
                    .frame(maxWidth: .infinity, maxHeight: .infinity)
            } else if sessionStore.libraryCards.isEmpty {
                ScrollView {
                    VStack(alignment: .leading, spacing: 16) {
                        statusMessages
                        Text("No cards yet")
                            .font(.largeTitle.bold())
                        Text("Use Add to create your first vocabulary card.")
                            .foregroundStyle(.secondary)
                    }
                    .padding()
                }
                .refreshable {
                    await sessionStore.loadLibraryCards()
                }
            } else {
                List {
                    statusMessages
                    ForEach(sessionStore.libraryCards) { card in
                        libraryRow(card)
                    }
                }
                .refreshable {
                    await sessionStore.loadLibraryCards()
                }
            }
        }
        .navigationTitle("Library")
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

    private func libraryRow(_ card: DueCard) -> some View {
        VStack(alignment: .leading, spacing: 10) {
            HStack(alignment: .firstTextBaseline) {
                Text(card.item.term)
                    .font(.headline)
                Spacer()
                Text(card.state.status)
                    .font(.caption.weight(.semibold))
                    .padding(.horizontal, 8)
                    .padding(.vertical, 4)
                    .background(.thinMaterial, in: Capsule())
            }

            Text(card.item.meaning.isEmpty ? "Meaning not added yet." : card.item.meaning)
                .foregroundStyle(.primary)

            Text("Next due: \(formattedDate(card.state.next_due_at))")
                .font(.caption)
                .foregroundStyle(.secondary)

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
        .padding(.vertical, 6)
    }

    private func formattedDate(_ value: String) -> String {
        guard let date = ISO8601DateFormatter.vocabReview.date(from: value) else {
            return value
        }
        return date.formatted(date: .abbreviated, time: .shortened)
    }
}
