import SwiftUI
import UIKit

enum AppTheme {
    static let displayFontName = "Cormorant Garamond"
    static let uiFontName = "DM Sans"

    static let linen = Color(red: 0.96, green: 0.94, blue: 0.88)
    static let paper = Color(red: 1.0, green: 1.0, blue: 0.98)
    static let paperStrong = Color(red: 1.0, green: 1.0, blue: 0.98)
    static let blush = Color(red: 0.98, green: 0.95, blue: 0.89)
    static let coral = Color(red: 0.55, green: 0.39, blue: 0.29)
    static let sand = Color(red: 0.77, green: 0.66, blue: 0.51)
    static let ink = Color(red: 0.17, green: 0.09, blue: 0.06)
    static let muted = Color(red: 0.55, green: 0.45, blue: 0.33)
    static let line = Color(red: 0.70, green: 0.55, blue: 0.45).opacity(0.22)
    static let lineStrong = Color(red: 0.55, green: 0.39, blue: 0.29).opacity(0.72)
    static let surfaceGlow = Color(red: 0.91, green: 0.84, blue: 0.70)
    static let shadow = Color(red: 0.44, green: 0.32, blue: 0.24)
    static let sage = coral
    static let sageDark = Color(red: 0.55, green: 0.42, blue: 0.33)
    static let clay = coral
    static let rose100 = Color(red: 0.92, green: 0.84, blue: 0.74)
    static let success = Color(red: 0.49, green: 0.69, blue: 0.51)
    static let successDark = Color(red: 0.31, green: 0.50, blue: 0.34)
    static let successPale = Color(red: 0.80, green: 0.91, blue: 0.81)
    static let successWash = Color(red: 0.91, green: 0.97, blue: 0.91)
    static let danger = Color(red: 0.71, green: 0.35, blue: 0.28)
    static let reviewGradientStart = Color(red: 0.55, green: 0.38, blue: 0.28)
    static let reviewGradientEnd = Color(red: 0.77, green: 0.66, blue: 0.51)

    static func displayFont(size: CGFloat, weight: Font.Weight = .semibold, relativeTo textStyle: Font.TextStyle = .largeTitle) -> Font {
        .custom(displayFontName, size: size, relativeTo: textStyle).weight(weight)
    }

    static func uiFont(size: CGFloat, weight: Font.Weight = .regular, relativeTo textStyle: Font.TextStyle = .body) -> Font {
        .custom(uiFontName, size: size, relativeTo: textStyle).weight(weight)
    }
}

struct ReadingDeskBackground: View {
    var body: some View {
        ZStack {
            LinearGradient(
                colors: [
                    AppTheme.linen,
                    AppTheme.blush,
                    AppTheme.linen
                ],
                startPoint: .top,
                endPoint: .bottom
            )

            RadialGradient(
                colors: [AppTheme.surfaceGlow.opacity(0.42), .clear],
                center: UnitPoint(x: 0.14, y: 0.1),
                startRadius: 8,
                endRadius: 240
            )

            RadialGradient(
                colors: [AppTheme.linen.opacity(0.76), .clear],
                center: UnitPoint(x: 0.84, y: 0.18),
                startRadius: 12,
                endRadius: 260
            )
        }
        .ignoresSafeArea()
    }
}

struct ReadingCard: ViewModifier {
    func body(content: Content) -> some View {
        content
            .padding()
            .background {
                RoundedRectangle(cornerRadius: 24, style: .continuous)
                    .fill(
                        LinearGradient(
                            colors: [
                                AppTheme.paperStrong,
                                AppTheme.paper.opacity(0.96)
                            ],
                            startPoint: .topLeading,
                            endPoint: .bottomTrailing
                        )
                    )
            }
            .overlay {
                RoundedRectangle(cornerRadius: 24, style: .continuous)
                    .stroke(AppTheme.line, lineWidth: 1)
            }
            .shadow(color: AppTheme.shadow.opacity(0.14), radius: 20, x: 0, y: 12)
            .shadow(color: AppTheme.coral.opacity(0.12), radius: 2, x: 0, y: 1)
    }
}

struct ReadingInputField: ViewModifier {
    func body(content: Content) -> some View {
        content
            .textFieldStyle(.plain)
            .font(AppTheme.uiFont(size: 17, weight: .medium))
            .foregroundStyle(AppTheme.ink)
            .colorScheme(.light)
            .tint(AppTheme.coral)
            .padding(.horizontal, 12)
            .padding(.vertical, 11)
            .background(AppTheme.paper.opacity(0.9), in: RoundedRectangle(cornerRadius: 12, style: .continuous))
            .overlay {
                RoundedRectangle(cornerRadius: 12, style: .continuous)
                    .stroke(AppTheme.line, lineWidth: 1)
            }
            .shadow(color: AppTheme.shadow.opacity(0.06), radius: 10, x: 0, y: 6)
    }
}

struct ReadingPrimaryButtonStyle: ButtonStyle {
    @Environment(\.isEnabled) private var isEnabled

    func makeBody(configuration: Configuration) -> some View {
        configuration.label
            .font(AppTheme.uiFont(size: 17, weight: .bold))
            .foregroundStyle(isEnabled ? AppTheme.paper : AppTheme.muted.opacity(0.72))
            .padding(.vertical, 13)
            .frame(maxWidth: .infinity)
            .background(
                isEnabled
                    ? AppTheme.coral.opacity(configuration.isPressed ? 0.78 : 1.0)
                    : AppTheme.rose100.opacity(0.42),
                in: RoundedRectangle(cornerRadius: 16, style: .continuous)
            )
            .overlay {
                RoundedRectangle(cornerRadius: 16, style: .continuous)
                    .stroke(isEnabled ? AppTheme.lineStrong : AppTheme.ink.opacity(0.06), lineWidth: 1)
            }
            .shadow(color: isEnabled ? AppTheme.coral.opacity(0.22) : .clear, radius: 10, x: 0, y: 6)
            .opacity(configuration.isPressed ? 0.92 : 1)
            .animation(.easeOut(duration: 0.12), value: configuration.isPressed)
            .animation(.easeOut(duration: 0.12), value: isEnabled)
    }
}

extension View {
    func readingCard() -> some View {
        modifier(ReadingCard())
    }

    func readingInputField() -> some View {
        modifier(ReadingInputField())
    }

    func readingPrimaryButton() -> some View {
        buttonStyle(ReadingPrimaryButtonStyle())
    }

    func readingTitle() -> some View {
        font(AppTheme.displayFont(size: 38, weight: .semibold))
            .foregroundStyle(AppTheme.ink)
    }

    func readingTerm() -> some View {
        font(AppTheme.displayFont(size: 28, weight: .semibold, relativeTo: .title2))
            .foregroundStyle(AppTheme.ink)
    }

    func readingMuted() -> some View {
        foregroundStyle(AppTheme.muted)
    }

    func dismissKeyboardOnTapOutside() -> some View {
        background(KeyboardDismissTapInstaller().frame(width: 0, height: 0))
    }
}

private struct KeyboardDismissTapInstaller: UIViewRepresentable {
    func makeCoordinator() -> Coordinator {
        Coordinator()
    }

    func makeUIView(context: Context) -> UIView {
        let view = UIView(frame: .zero)
        view.isUserInteractionEnabled = false
        DispatchQueue.main.async {
            context.coordinator.install(in: view.window)
        }
        return view
    }

    func updateUIView(_ uiView: UIView, context: Context) {
        DispatchQueue.main.async {
            context.coordinator.install(in: uiView.window)
        }
    }

    static func dismantleUIView(_ uiView: UIView, coordinator: Coordinator) {
        coordinator.uninstall()
    }

    final class Coordinator: NSObject, UIGestureRecognizerDelegate {
        private weak var gesture: UITapGestureRecognizer?

        func install(in window: UIWindow?) {
            guard let window, gesture?.view !== window else { return }
            uninstall()
            let tapGesture = UITapGestureRecognizer(target: self, action: #selector(dismissKeyboard))
            tapGesture.cancelsTouchesInView = false
            tapGesture.delegate = self
            window.addGestureRecognizer(tapGesture)
            gesture = tapGesture
        }

        func uninstall() {
            guard let gesture else { return }
            gesture.view?.removeGestureRecognizer(gesture)
            self.gesture = nil
        }

        @objc private func dismissKeyboard() {
            UIApplication.shared.sendAction(#selector(UIResponder.resignFirstResponder), to: nil, from: nil, for: nil)
        }

        func gestureRecognizer(_ gestureRecognizer: UIGestureRecognizer, shouldReceive touch: UITouch) -> Bool {
            var view = touch.view
            while let currentView = view {
                if currentView is UITextField || currentView is UITextView {
                    return false
                }
                view = currentView.superview
            }
            return true
        }
    }
}
