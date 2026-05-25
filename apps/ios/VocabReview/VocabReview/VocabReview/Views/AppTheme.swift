import SwiftUI

enum AppTheme {
    static let linen = Color(red: 1.0, green: 0.89, blue: 0.88)
    static let paper = Color(red: 1.0, green: 0.96, blue: 0.89)
    static let blush = Color(red: 1.0, green: 0.82, blue: 0.82)
    static let coral = Color(red: 1.0, green: 0.58, blue: 0.58)
    static let ink = Color(red: 0.26, green: 0.15, blue: 0.16)
    static let muted = Color(red: 0.46, green: 0.34, blue: 0.35)
    static let sage = coral
    static let sageDark = Color(red: 0.65, green: 0.30, blue: 0.32)
    static let clay = coral
    static let success = Color(red: 0.34, green: 0.72, blue: 0.48)
    static let successDark = Color(red: 0.16, green: 0.45, blue: 0.27)
    static let danger = Color(red: 0.78, green: 0.26, blue: 0.29)
}

struct ReadingDeskBackground: View {
    var body: some View {
        LinearGradient(
            colors: [
                AppTheme.paper,
                AppTheme.linen,
                AppTheme.paper
            ],
            startPoint: .topLeading,
            endPoint: .bottomTrailing
        )
        .overlay(alignment: .topLeading) {
            LinearGradient(
                colors: [AppTheme.linen.opacity(0.72), .clear],
                startPoint: .topLeading,
                endPoint: .bottomTrailing
            )
            .frame(width: 280, height: 220)
            .offset(x: -60, y: -40)
        }
        .overlay(alignment: .topTrailing) {
            LinearGradient(
                colors: [AppTheme.blush.opacity(0.62), .clear],
                startPoint: .topTrailing,
                endPoint: .bottomLeading
            )
            .frame(width: 260, height: 220)
            .offset(x: 60, y: -36)
        }
        .ignoresSafeArea()
    }
}

struct ReadingCard: ViewModifier {
    func body(content: Content) -> some View {
        content
            .padding()
            .background(AppTheme.paper.opacity(0.88), in: RoundedRectangle(cornerRadius: 24, style: .continuous))
            .overlay {
                RoundedRectangle(cornerRadius: 24, style: .continuous)
                    .stroke(AppTheme.coral.opacity(0.18), lineWidth: 1)
            }
            .shadow(color: AppTheme.sageDark.opacity(0.12), radius: 22, x: 0, y: 12)
    }
}

extension View {
    func readingCard() -> some View {
        modifier(ReadingCard())
    }

    func readingTitle() -> some View {
        font(.system(.largeTitle, design: .serif, weight: .semibold))
            .foregroundStyle(AppTheme.ink)
    }

    func readingTerm() -> some View {
        font(.system(.title, design: .serif, weight: .semibold))
            .foregroundStyle(AppTheme.ink)
    }

    func readingMuted() -> some View {
        foregroundStyle(AppTheme.muted)
    }
}
