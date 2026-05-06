import SwiftUI

struct HistoryView: View {
    @EnvironmentObject private var sessionStore: SessionStore
    @State private var selectedEntry: ReviewHistoryEntry?

    var body: some View {
        ZStack {
            ReadingDeskBackground()

            if sessionStore.isLoadingReviewHistory && sessionStore.reviewHistory.isEmpty {
                ProgressView("Loading history...")
                    .padding()
                    .readingCard()
            } else {
                ScrollView {
                    VStack(alignment: .leading, spacing: 16) {
                        statusMessages
                        header
                        historyCards
                    }
                    .padding()
                }
                .refreshable {
                    await sessionStore.loadReviewHistory()
                }
            }
        }
        .navigationTitle("History")
        .sheet(item: $selectedEntry) { entry in
            HistoryDetailSheet(entry: entry)
        }
    }

    private var header: some View {
        VStack(alignment: .leading, spacing: 8) {
            Text("Recent reviews")
                .readingTitle()
            Text("Tap a card to inspect the full review record.")
                .readingMuted()
        }
        .readingCard()
    }

    @ViewBuilder
    private var historyCards: some View {
        if sessionStore.reviewHistory.isEmpty {
            VStack(alignment: .leading, spacing: 8) {
                Text("No reviews yet")
                    .readingTerm()
                Text("Review a due card and it will appear here.")
                    .readingMuted()
            }
            .readingCard()
        } else {
            LazyVGrid(columns: [GridItem(.adaptive(minimum: 150), spacing: 12)], spacing: 12) {
                ForEach(sessionStore.reviewHistory) { entry in
                    Button {
                        selectedEntry = entry
                    } label: {
                        VStack(alignment: .leading, spacing: 12) {
                            Text(entry.item.term)
                                .font(.system(.title3, design: .serif, weight: .semibold))
                                .foregroundStyle(AppTheme.ink)
                                .lineLimit(2)

                            Spacer(minLength: 8)

                            HStack(spacing: 8) {
                                Text(entry.state.status.capitalized)
                                    .font(.caption.weight(.semibold))
                                    .padding(.horizontal, 8)
                                    .padding(.vertical, 5)
                                    .foregroundStyle(AppTheme.sageDark)
                                    .background(AppTheme.sage.opacity(0.12), in: Capsule())

                                if entry.item.archived_at != nil {
                                    Text("Archived")
                                        .font(.caption2.weight(.bold))
                                        .padding(.horizontal, 7)
                                        .padding(.vertical, 4)
                                        .foregroundStyle(AppTheme.danger)
                                        .background(AppTheme.danger.opacity(0.1), in: Capsule())
                                }
                            }
                        }
                        .frame(maxWidth: .infinity, minHeight: 128, alignment: .topLeading)
                        .padding()
                        .background(AppTheme.paper.opacity(0.9), in: RoundedRectangle(cornerRadius: 22, style: .continuous))
                        .overlay {
                            RoundedRectangle(cornerRadius: 22, style: .continuous)
                                .stroke(selectedEntry?.id == entry.id ? AppTheme.sage.opacity(0.45) : AppTheme.ink.opacity(0.08), lineWidth: selectedEntry?.id == entry.id ? 2 : 1)
                        }
                    }
                    .buttonStyle(.plain)
                }
            }
        }
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

    private func formattedDate(_ value: String) -> String {
        guard let date = ISO8601DateFormatter.vocabReview.date(from: value) else {
            return value
        }
        return date.formatted(date: .abbreviated, time: .shortened)
    }
}

private struct HistoryDetailSheet: View {
    let entry: ReviewHistoryEntry
    @Environment(\.dismiss) private var dismiss

    var body: some View {
        NavigationStack {
            ZStack {
                ReadingDeskBackground()

                ScrollView {
                    VStack(alignment: .leading, spacing: 18) {
                        VStack(alignment: .leading, spacing: 16) {
                            Text(entry.item.term)
                                .font(.system(size: 42, weight: .semibold, design: .serif))
                                .lineSpacing(4)
                                .fixedSize(horizontal: false, vertical: true)
                                .foregroundStyle(AppTheme.ink)
                            Text(entry.item.meaning.isEmpty ? "Meaning not added yet." : entry.item.meaning)
                                .font(.title3)
                                .lineSpacing(5)
                                .fixedSize(horizontal: false, vertical: true)
                        }
                        .padding(24)
                        .frame(maxWidth: .infinity, alignment: .leading)
                        .background(AppTheme.paper.opacity(0.9), in: RoundedRectangle(cornerRadius: 28, style: .continuous))
                        .overlay {
                            RoundedRectangle(cornerRadius: 28, style: .continuous)
                                .stroke(AppTheme.ink.opacity(0.08), lineWidth: 1)
                        }

                        if !entry.item.example_sentence.isEmpty {
                            VStack(alignment: .leading, spacing: 8) {
                                Text("Example")
                                    .font(.headline)
                                Text(entry.item.example_sentence)
                                    .readingMuted()
                            }
                            .readingCard()
                        }

                        if !entry.item.notes.isEmpty {
                            VStack(alignment: .leading, spacing: 8) {
                                Text("Notes")
                                    .font(.headline)
                                Text(entry.item.notes)
                                    .readingMuted()
                            }
                            .readingCard()
                        }

                        DetailRows(rows: [
                            DetailRow(label: "Grade", value: entry.log.grade.capitalized),
                            DetailRow(label: "Status", value: entry.state.status.capitalized),
                            DetailRow(label: "Review date", value: formattedDate(entry.log.reviewed_at)),
                            DetailRow(label: "Next due", value: formattedDate(entry.state.next_due_at))
                        ], isArchived: entry.item.archived_at != nil)
                        .readingCard()
                    }
                    .padding(.horizontal)
                    .padding(.top, 22)
                    .padding(.bottom)
                }
            }
            .navigationTitle("Review detail")
            .toolbar {
                ToolbarItem(placement: .confirmationAction) {
                    Button("Done") {
                        dismiss()
                    }
                }
            }
        }
    }

    private func formattedDate(_ value: String) -> String {
        guard let date = ISO8601DateFormatter.vocabReview.date(from: value) else {
            return value
        }
        return date.formatted(date: .abbreviated, time: .shortened)
    }
}

private struct DetailRow: Identifiable {
    let label: String
    let value: String

    var id: String { label }
}

private struct DetailRows: View {
    let rows: [DetailRow]
    let isArchived: Bool

    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            ForEach(rows) { row in
                VStack(alignment: .leading, spacing: 4) {
                    Text(row.label)
                        .font(.caption.weight(.semibold))
                        .foregroundStyle(AppTheme.muted)
                    Text(row.value)
                        .foregroundStyle(AppTheme.ink)
                }
                .frame(maxWidth: .infinity, alignment: .leading)
            }

            if isArchived {
                VStack(alignment: .leading, spacing: 4) {
                    Text("Archive")
                        .font(.caption.weight(.semibold))
                        .foregroundStyle(AppTheme.muted)
                    Text("Archived")
                        .foregroundStyle(AppTheme.danger)
                }
                .frame(maxWidth: .infinity, alignment: .leading)
            }
        }
    }
}
