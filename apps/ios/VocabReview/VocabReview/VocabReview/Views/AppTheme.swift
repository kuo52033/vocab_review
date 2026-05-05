import SwiftUI

enum AppTheme {
    static let linen = Color(red: 0.94, green: 0.89, blue: 0.81)
    static let paper = Color(red: 1.0, green: 0.97, blue: 0.91)
    static let ink = Color(red: 0.15, green: 0.14, blue: 0.11)
    static let muted = Color(red: 0.44, green: 0.41, blue: 0.37)
    static let sage = Color(red: 0.40, green: 0.48, blue: 0.37)
    static let sageDark = Color(red: 0.25, green: 0.33, blue: 0.24)
    static let clay = Color(red: 0.73, green: 0.39, blue: 0.25)
    static let danger = Color(red: 0.62, green: 0.26, blue: 0.21)
}

struct ReadingDeskBackground: View {
    var body: some View {
        LinearGradient(
            colors: [
                Color(red: 0.97, green: 0.93, blue: 0.86),
                AppTheme.linen,
                Color(red: 0.93, green: 0.90, blue: 0.84)
            ],
            startPoint: .topLeading,
            endPoint: .bottomTrailing
        )
        .overlay(alignment: .topLeading) {
            Circle()
                .fill(AppTheme.clay.opacity(0.16))
                .frame(width: 260, height: 260)
                .blur(radius: 18)
                .offset(x: -100, y: -90)
        }
        .overlay(alignment: .topTrailing) {
            Circle()
                .fill(AppTheme.sage.opacity(0.14))
                .frame(width: 230, height: 230)
                .blur(radius: 20)
                .offset(x: 90, y: -70)
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
                    .stroke(AppTheme.ink.opacity(0.08), lineWidth: 1)
            }
            .shadow(color: AppTheme.ink.opacity(0.08), radius: 22, x: 0, y: 12)
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
