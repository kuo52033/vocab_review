import SwiftUI

struct RootView: View {
    @EnvironmentObject private var sessionStore: SessionStore
    @Environment(\.scenePhase) private var scenePhase
    @State private var email = ""
    @State private var magicToken = ""
    @State private var selectedTab: AppTab = .review
    @State private var isReviewSessionActive = false
    @Namespace private var tabSelectionNamespace
    @FocusState private var signInFocusedField: SignInField?

    var body: some View {
        NavigationStack {
            if sessionStore.isAuthenticated {
                authenticatedView
            } else {
                signInView
            }
        }
        .tint(AppTheme.sage)
        .dismissKeyboardOnTapOutside()
        .onChange(of: scenePhase) { _, newPhase in
            guard newPhase == .active, sessionStore.isAuthenticated else { return }
            Task { await sessionStore.refreshAuthenticatedData() }
        }
    }

    private var authenticatedView: some View {
        ZStack {
            ReadingDeskBackground()

            VStack(spacing: 0) {
                if !isReviewSessionActive {
                    authenticatedActionBar
                        .padding(.horizontal)
                        .padding(.top, 8)
                        .padding(.bottom, 8)
                        .transition(.opacity.combined(with: .move(edge: .top)))
                }

                ZStack(alignment: .top) {
                    ReviewListView(isReviewSessionActive: $isReviewSessionActive)
                        .opacity(selectedTab == .review ? 1 : 0)
                        .allowsHitTesting(selectedTab == .review)
                        .accessibilityHidden(selectedTab != .review)

                    LibraryView()
                        .opacity(selectedTab == .library ? 1 : 0)
                        .allowsHitTesting(selectedTab == .library)
                        .accessibilityHidden(selectedTab != .library)

                    AddCardsView()
                        .opacity(selectedTab == .add ? 1 : 0)
                        .allowsHitTesting(selectedTab == .add)
                        .accessibilityHidden(selectedTab != .add)
                }
            }
        }
        .toolbar(.hidden, for: .navigationBar)
        .safeAreaInset(edge: .bottom) {
            if !isReviewSessionActive {
                authenticatedTabBar
                    .transition(.opacity.combined(with: .move(edge: .bottom)))
            }
        }
        .animation(.spring(response: 0.34, dampingFraction: 0.86), value: selectedTab)
        .animation(.easeOut(duration: 0.22), value: isReviewSessionActive)
        .task { await sessionStore.refreshAuthenticatedData() }
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
                        .foregroundStyle(AppTheme.ink)
                    TextField("you@example.com", text: $email)
                        .textInputAutocapitalization(.never)
                        .autocorrectionDisabled()
                        .keyboardType(.emailAddress)
                        .readingInputField()
                        .focused($signInFocusedField, equals: .email)
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
                        if let verificationURL = link.verification_url, let token = link.token {
                            Text("Development link")
                                .font(.headline)
                            Text(verificationURL)
                                .textSelection(.enabled)
                                .font(.footnote.monospaced())
                            Text("Token: \(token)")
                                .textSelection(.enabled)
                                .font(.footnote.monospaced())

                            HStack {
                                Button("Use this link") {
                                    Task { await sessionStore.useRequestedMagicLink() }
                                }
                                .buttonStyle(.borderedProminent)
                                .disabled(sessionStore.isSigningIn || sessionStore.isRequestingMagicLink)

                                Button("Fill token") {
                                    magicToken = token
                                }
                                .buttonStyle(.bordered)
                            }
                        } else {
                            Text(link.message)
                                .foregroundStyle(AppTheme.muted)
                        }
                    }
                    .readingCard()
                }

                VStack(alignment: .leading, spacing: 12) {
                    Text("Paste token fallback")
                        .font(.headline)
                        .foregroundStyle(AppTheme.ink)
                    TextField("Paste magic token", text: $magicToken)
                        .textInputAutocapitalization(.never)
                        .autocorrectionDisabled()
                        .readingInputField()
                        .focused($signInFocusedField, equals: .magicToken)
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

    private var authenticatedActionBar: some View {
        HStack {
            Spacer()

            HStack(spacing: 8) {
                topActionButton("Notify") {
                    Task { await sessionStore.registerNotifications() }
                }
                topActionButton("Sign out") {
                    sessionStore.signOut()
                }
            }
            .padding(5)
            .background(AppTheme.paper.opacity(0.78), in: Capsule())
            .overlay {
                Capsule()
                    .stroke(AppTheme.line, lineWidth: 1)
            }
            .shadow(color: AppTheme.shadow.opacity(0.08), radius: 14, x: 0, y: 8)

            Spacer()
        }
    }

    private var authenticatedTabBar: some View {
        HStack(spacing: 0) {
            tabButton(.review, title: "Review", systemImage: "rectangle.stack.fill")
            tabButton(.add, title: "Add", systemImage: "plus.square.fill")
            tabButton(.library, title: "Library", systemImage: "books.vertical.fill")
        }
        .padding(.horizontal, 18)
        .padding(.top, 8)
        .padding(.bottom, 6)
        .frame(maxWidth: .infinity)
        .background(AppTheme.blush.opacity(0.96))
        .overlay(alignment: .top) {
            Rectangle()
                .fill(AppTheme.coral.opacity(0.28))
                .frame(height: 1.5)
        }
    }

    private func tabButton(_ tab: AppTab, title: String, systemImage: String) -> some View {
        Button {
            withAnimation(.spring(response: 0.38, dampingFraction: 0.82)) {
                selectedTab = tab
            }
        } label: {
            VStack(spacing: 4) {
                Image(systemName: systemImage)
                    .font(AppTheme.uiFont(size: 20, weight: .semibold, relativeTo: .title3))
                    .frame(width: 50, height: 42)
                    .background {
                        if selectedTab == tab {
                            RoundedRectangle(cornerRadius: 16, style: .continuous)
                                .fill(AppTheme.rose100.opacity(0.92))
                                .matchedGeometryEffect(id: "selectedTabBackground", in: tabSelectionNamespace)
                        }
                    }
                    .scaleEffect(selectedTab == tab ? 1.06 : 1.0)
                    .shadow(color: selectedTab == tab ? AppTheme.shadow.opacity(0.12) : .clear, radius: 10, x: 0, y: 5)
                Text(title)
                    .font(AppTheme.uiFont(size: 12, weight: .semibold, relativeTo: .caption))
                    .opacity(selectedTab == tab ? 1 : 0.78)
            }
            .frame(maxWidth: .infinity, minHeight: 64)
        }
        .buttonStyle(ReadingTabButtonStyle(isSelected: selectedTab == tab))
        .accessibilityLabel(title)
        .accessibilityAddTraits(selectedTab == tab ? .isSelected : [])
    }

    private func topActionButton(_ title: String, action: @escaping () -> Void) -> some View {
        Button(action: action) {
            Text(title)
                .font(AppTheme.uiFont(size: 16, weight: .medium, relativeTo: .callout))
                .lineLimit(1)
                .minimumScaleFactor(0.82)
                .frame(minWidth: 64, minHeight: 36)
        }
        .buttonStyle(ReadingToolbarButtonStyle())
        .accessibilityLabel(title)
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

private enum AppTab: Hashable {
    case review
    case add
    case library
}

private enum SignInField: Hashable {
    case email
    case magicToken
}

private struct ReadingToolbarButtonStyle: ButtonStyle {
    func makeBody(configuration: Configuration) -> some View {
        configuration.label
            .foregroundStyle(AppTheme.coral)
            .background(
                AppTheme.rose100.opacity(configuration.isPressed ? 0.36 : 0.0),
                in: Capsule()
            )
            .opacity(configuration.isPressed ? 0.82 : 1.0)
            .animation(.easeOut(duration: 0.12), value: configuration.isPressed)
    }
}

private struct ReadingTabButtonStyle: ButtonStyle {
    let isSelected: Bool

    func makeBody(configuration: Configuration) -> some View {
        configuration.label
            .foregroundStyle(isSelected ? AppTheme.coral : AppTheme.muted)
            .opacity(configuration.isPressed ? 0.82 : 1.0)
            .animation(.easeOut(duration: 0.12), value: configuration.isPressed)
            .animation(.easeOut(duration: 0.18), value: isSelected)
    }
}
