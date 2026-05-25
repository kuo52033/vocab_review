import SwiftUI
import UIKit

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

struct ReadingInputField: ViewModifier {
    func body(content: Content) -> some View {
        content
            .textFieldStyle(.plain)
            .font(.body.weight(.medium))
            .foregroundStyle(AppTheme.ink)
            .tint(AppTheme.coral)
            .padding(.horizontal, 12)
            .padding(.vertical, 11)
            .background(AppTheme.paper.opacity(0.96), in: RoundedRectangle(cornerRadius: 8, style: .continuous))
            .overlay {
                RoundedRectangle(cornerRadius: 8, style: .continuous)
                    .stroke(AppTheme.ink.opacity(0.1), lineWidth: 1)
            }
    }
}

struct ReadingPrimaryButtonStyle: ButtonStyle {
    @Environment(\.isEnabled) private var isEnabled

    func makeBody(configuration: Configuration) -> some View {
        configuration.label
            .font(.body.weight(.bold))
            .foregroundStyle(isEnabled ? Color.white : AppTheme.muted.opacity(0.72))
            .padding(.vertical, 13)
            .frame(maxWidth: .infinity)
            .background(
                isEnabled
                    ? AppTheme.coral.opacity(configuration.isPressed ? 0.78 : 1.0)
                    : AppTheme.blush.opacity(0.42),
                in: RoundedRectangle(cornerRadius: 16, style: .continuous)
            )
            .overlay {
                RoundedRectangle(cornerRadius: 16, style: .continuous)
                    .stroke(isEnabled ? Color.white.opacity(0.28) : AppTheme.ink.opacity(0.06), lineWidth: 1)
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
