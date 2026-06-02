import SwiftUI

struct AddCardsView: View {
    @State private var addMode: AddCardsMode = .single

    var body: some View {
        ZStack {
            ReadingDeskBackground()

            ScrollView {
                VStack(alignment: .leading, spacing: 18) {
                    modePicker

                    Group {
                        switch addMode {
                        case .single:
                            AddVocabView(presentation: .embedded)
                        case .bulk:
                            BulkImportView(presentation: .embedded)
                        }
                    }
                }
                .padding()
                .padding(.top, 24)
                .padding(.bottom, 80)
            }
        }
    }

    private var modePicker: some View {
        HStack(spacing: 0) {
            modeButton(.single, title: "Single card", systemImage: "rectangle.stack.badge.plus")
            modeButton(.bulk, title: "Bulk import", systemImage: "square.and.arrow.down.on.square")
        }
        .padding(5)
        .background(AppTheme.paper, in: Capsule())
        .overlay {
            Capsule()
                .stroke(AppTheme.line, lineWidth: 1)
        }
        .shadow(color: AppTheme.shadow.opacity(0.08), radius: 14, x: 0, y: 6)
    }

    private func modeButton(_ mode: AddCardsMode, title: String, systemImage: String) -> some View {
        Button {
            withAnimation(.easeOut(duration: 0.18)) {
                addMode = mode
            }
        } label: {
            Label(title, systemImage: systemImage)
                .font(.callout.weight(.semibold))
                .lineLimit(1)
                .minimumScaleFactor(0.82)
                .frame(maxWidth: .infinity, minHeight: 42)
        }
        .buttonStyle(ReadingSegmentButtonStyle(isSelected: addMode == mode))
        .accessibilityAddTraits(addMode == mode ? .isSelected : [])
    }
}

enum AddCardsPresentation {
    case standalone
    case embedded
}

private enum AddCardsMode {
    case single
    case bulk
}

private struct ReadingSegmentButtonStyle: ButtonStyle {
    let isSelected: Bool

    func makeBody(configuration: Configuration) -> some View {
        configuration.label
            .foregroundStyle(isSelected ? AppTheme.paper : AppTheme.ink)
            .background(
                isSelected ? AppTheme.coral.opacity(configuration.isPressed ? 0.86 : 1.0) : AppTheme.blush.opacity(configuration.isPressed ? 0.28 : 0.0),
                in: Capsule()
            )
            .opacity(configuration.isPressed ? 0.88 : 1)
            .animation(.easeOut(duration: 0.12), value: configuration.isPressed)
            .animation(.easeOut(duration: 0.18), value: isSelected)
    }
}
