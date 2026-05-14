import SwiftUI

struct RootView: View {
    @EnvironmentObject private var sessionStore: SessionStore
    @Environment(\.scenePhase) private var scenePhase
    @State private var email = ""
    @State private var magicToken = ""
    @State private var isAddingVocab = false
    @State private var isImportingVocab = false

    var body: some View {
        NavigationStack {
            if sessionStore.isAuthenticated {
                ZStack(alignment: .top) {
                    TabView {
                        ReviewListView()
                            .tabItem {
                                Label("Review", systemImage: "rectangle.stack")
                            }

                        LibraryView()
                            .tabItem {
                                Label("Library", systemImage: "books.vertical")
                            }
                    }
                    .tint(AppTheme.sage)

                    if !sessionStore.errorMessage.isEmpty || !sessionStore.infoMessage.isEmpty {
                        authenticatedStatusBanner
                            .padding(.horizontal)
                            .padding(.top, 8)
                            .transition(.move(edge: .top).combined(with: .opacity))
                            .zIndex(1)
                    }
                }
                .animation(.easeInOut(duration: 0.2), value: sessionStore.errorMessage)
                .animation(.easeInOut(duration: 0.2), value: sessionStore.infoMessage)
                .task { await sessionStore.refreshAuthenticatedData() }
                .toolbar {
                    ToolbarItemGroup(placement: .topBarTrailing) {
                        Button("Add") {
                            isAddingVocab = true
                        }
                        Button("Import") {
                            isImportingVocab = true
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
        .sheet(isPresented: $isImportingVocab) {
            BulkImportView()
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

    private var authenticatedStatusBanner: some View {
        HStack(alignment: .top, spacing: 10) {
            Image(systemName: sessionStore.errorMessage.isEmpty ? "bell.badge" : "exclamationmark.triangle")
                .foregroundStyle(sessionStore.errorMessage.isEmpty ? AppTheme.sageDark : AppTheme.danger)

            VStack(alignment: .leading, spacing: 4) {
                if !sessionStore.errorMessage.isEmpty {
                    Text(sessionStore.errorMessage)
                        .foregroundStyle(AppTheme.danger)
                }
                if !sessionStore.infoMessage.isEmpty {
                    Text(sessionStore.infoMessage)
                        .foregroundStyle(AppTheme.ink)
                }
            }
            .font(.callout.weight(.semibold))

            Spacer()

            Button("Dismiss") {
                sessionStore.clearError()
                sessionStore.infoMessage = ""
            }
            .font(.caption.weight(.semibold))
            .buttonStyle(.bordered)
        }
        .padding()
        .background(AppTheme.paper.opacity(0.95), in: RoundedRectangle(cornerRadius: 18, style: .continuous))
        .overlay {
            RoundedRectangle(cornerRadius: 18, style: .continuous)
                .stroke(AppTheme.ink.opacity(0.08), lineWidth: 1)
        }
        .shadow(color: AppTheme.ink.opacity(0.08), radius: 14, x: 0, y: 8)
    }
}
