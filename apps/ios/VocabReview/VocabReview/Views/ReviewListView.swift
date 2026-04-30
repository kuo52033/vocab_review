import SwiftUI

struct ReviewListView: View {
    @EnvironmentObject private var sessionStore: SessionStore

    var body: some View {
        List {
            if sessionStore.dueCards.isEmpty {
                Text("No cards due right now.")
            }

            ForEach(sessionStore.dueCards) { card in
                Section(card.item.term) {
                    Text(card.item.meaning.isEmpty ? "Meaning not added yet." : card.item.meaning)
                    if !card.item.example_sentence.isEmpty {
                        Text(card.item.example_sentence)
                            .foregroundStyle(.secondary)
                    }
                    HStack {
                        ForEach(["again", "hard", "good", "easy"], id: \.self) { grade in
                            Button(grade.capitalized) {
                                Task { await sessionStore.grade(cardID: card.item.id, grade: grade) }
                            }
                            .buttonStyle(.bordered)
                        }
                    }
                }
            }
        }
        .navigationTitle("Due Review")
    }
}
