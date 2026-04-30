import SwiftUI

struct RootView: View {
    @EnvironmentObject private var sessionStore: SessionStore
    @State private var magicToken = ""

    var body: some View {
        NavigationStack {
            if sessionStore.sessionToken.isEmpty {
                VStack(alignment: .leading, spacing: 20) {
                    Text("Review before you forget")
                        .font(.largeTitle.bold())
                    TextField("Paste magic token", text: $magicToken)
                        .textFieldStyle(.roundedBorder)
                    Button("Sign in") {
                        Task { await sessionStore.signIn(with: magicToken) }
                    }
                    .buttonStyle(.borderedProminent)
                    if !sessionStore.errorMessage.isEmpty {
                        Text(sessionStore.errorMessage)
                            .foregroundStyle(.red)
                    }
                }
                .padding()
            } else {
                ReviewListView()
                    .task { await sessionStore.loadDueCards() }
                    .toolbar {
                        ToolbarItem(placement: .topBarTrailing) {
                            Button("Notify") {
                                Task { await sessionStore.registerNotifications() }
                            }
                        }
                    }
            }
        }
    }
}
