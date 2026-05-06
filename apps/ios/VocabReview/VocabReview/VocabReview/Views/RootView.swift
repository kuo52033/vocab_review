import SwiftUI

struct RootView: View {
    @EnvironmentObject private var sessionStore: SessionStore
    @Environment(\.scenePhase) private var scenePhase
    @State private var email = ""
    @State private var magicToken = ""
    @State private var isAddingVocab = false

    var body: some View {
        NavigationStack {
            if sessionStore.isAuthenticated {
                TabView {
                    ReviewListView()
                        .tabItem {
                            Label("Review", systemImage: "rectangle.stack")
                        }

                    LibraryView()
                        .tabItem {
                            Label("Library", systemImage: "books.vertical")
                        }

                    HistoryView()
                        .tabItem {
                            Label("History", systemImage: "clock.arrow.circlepath")
                        }
                }
                .tint(AppTheme.sage)
                .task { await sessionStore.refreshAuthenticatedData() }
                .toolbar {
                    ToolbarItemGroup(placement: .topBarTrailing) {
                        Button("Add") {
                            isAddingVocab = true
                        }
                        Button("Notify") {
                            Task { await sessionStore.registerNotifications() }
                        }
                        Button("Sign out") {
                            sessionStore.signOut()
                        }
                    }
                }
            } else {
                signInView
            }
        }
        .tint(AppTheme.sage)
        .sheet(isPresented: $isAddingVocab) {
            AddVocabView()
                .environmentObject(sessionStore)
        }
        .onChange(of: scenePhase) { _, newPhase in
            guard newPhase == .active, sessionStore.isAuthenticated else { return }
            Task { await sessionStore.refreshAuthenticatedData() }
        }
    }

    private var signInView: some View {
        ZStack {
            ReadingDeskBackground()

            ScrollView {
                VStack(alignment: .leading, spacing: 24) {
                VStack(alignment: .leading, spacing: 8) {
                    Text("Review before you forget")
                        .readingTitle()
                    Text("Request a development magic link, then verify it in-app.")
                        .readingMuted()
                }

                statusMessages

                VStack(alignment: .leading, spacing: 12) {
                    Text("Request magic link")
                        .font(.headline)
                    TextField("you@example.com", text: $email)
                        .textInputAutocapitalization(.never)
                        .autocorrectionDisabled()
                        .keyboardType(.emailAddress)
                        .textFieldStyle(.roundedBorder)
                    Button {
                        Task { await sessionStore.requestMagicLink(for: email) }
                    } label: {
                        if sessionStore.isRequestingMagicLink {
                            ProgressView()
                                .frame(maxWidth: .infinity)
                        } else {
                            Text("Request link")
                                .frame(maxWidth: .infinity)
                        }
                    }
                    .buttonStyle(.borderedProminent)
                    .disabled(sessionStore.isRequestingMagicLink || sessionStore.isSigningIn)
                }
                .readingCard()

                if let link = sessionStore.requestedMagicLink {
                    VStack(alignment: .leading, spacing: 12) {
                        Text("Development link")
                            .font(.headline)
                        Text(link.verification_url)
                            .textSelection(.enabled)
                            .font(.footnote.monospaced())
                        Text("Token: \(link.token)")
                            .textSelection(.enabled)
                            .font(.footnote.monospaced())

                        HStack {
                            Button("Use this link") {
                                Task { await sessionStore.useRequestedMagicLink() }
                            }
                            .buttonStyle(.borderedProminent)
                            .disabled(sessionStore.isSigningIn || sessionStore.isRequestingMagicLink)

                            Button("Fill token") {
                                magicToken = link.token
                            }
                            .buttonStyle(.bordered)
                        }
                    }
                    .readingCard()
                }

                VStack(alignment: .leading, spacing: 12) {
                    Text("Paste token fallback")
                        .font(.headline)
                    TextField("Paste magic token", text: $magicToken)
                        .textInputAutocapitalization(.never)
                        .autocorrectionDisabled()
                        .textFieldStyle(.roundedBorder)
                    Button {
                        Task { await sessionStore.signIn(with: magicToken) }
                    } label: {
                        if sessionStore.isSigningIn {
                            ProgressView()
                                .frame(maxWidth: .infinity)
                        } else {
                            Text("Verify token")
                                .frame(maxWidth: .infinity)
                        }
                    }
                    .buttonStyle(.bordered)
                    .disabled(sessionStore.isSigningIn || sessionStore.isRequestingMagicLink)
                }
                .readingCard()
            }
            .padding()
            }
        }
    }

    @ViewBuilder
    private var statusMessages: some View {
        if !sessionStore.errorMessage.isEmpty {
            Text(sessionStore.errorMessage)
                .foregroundStyle(.red)
        }
        if !sessionStore.infoMessage.isEmpty {
            Text(sessionStore.infoMessage)
                .foregroundStyle(.secondary)
        }
    }
}
