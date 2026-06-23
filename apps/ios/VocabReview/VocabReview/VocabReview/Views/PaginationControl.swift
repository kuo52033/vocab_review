import SwiftUI

struct PaginationControl: View {
    let page: Int
    let hasNext: Bool
    let previous: () -> Void
    let next: () -> Void

    var body: some View {
        if page > 1 || hasNext {
            HStack {
                Button(action: previous) {
                    Label("Previous", systemImage: "chevron.left")
                }
                .buttonStyle(PaginationButtonStyle())
                .disabled(page <= 1)

                Spacer()

                Text("Page \(page)")
                    .font(.footnote.weight(.semibold))
                    .foregroundStyle(AppTheme.sageDark)

                Spacer()

                Button(action: next) {
                    Label("Next", systemImage: "chevron.right")
                        .labelStyle(.titleAndIcon)
                }
                .buttonStyle(PaginationButtonStyle())
                .disabled(!hasNext)
            }
            .padding(.vertical, 6)
        }
    }
}

private struct PaginationButtonStyle: ButtonStyle {
    @Environment(\.isEnabled) private var isEnabled

    func makeBody(configuration: Configuration) -> some View {
        configuration.label
            .font(.footnote.weight(.semibold))
            .foregroundStyle(isEnabled ? AppTheme.coral : AppTheme.muted.opacity(0.45))
            .padding(.horizontal, 12)
            .padding(.vertical, 8)
            .background(
                AppTheme.blush.opacity(configuration.isPressed ? 0.42 : 0.22),
                in: Capsule()
            )
            .overlay {
                Capsule()
                    .stroke(AppTheme.coral.opacity(isEnabled ? 0.16 : 0.06), lineWidth: 1)
            }
            .opacity(isEnabled ? 1 : 0.62)
            .animation(.easeOut(duration: 0.12), value: configuration.isPressed)
            .animation(.easeOut(duration: 0.12), value: isEnabled)
    }
}
