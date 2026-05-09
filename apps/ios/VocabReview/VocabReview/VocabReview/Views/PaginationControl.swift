import SwiftUI

struct PaginationControl: View {
    let page: Int
    let totalPages: Int
    let previous: () -> Void
    let next: () -> Void

    var body: some View {
        if totalPages > 1 {
            HStack {
                Button("Previous", action: previous)
                    .buttonStyle(.bordered)
                    .disabled(page <= 1)

                Spacer()

                Text("Page \(page) of \(totalPages)")
                    .font(.footnote.weight(.semibold))
                    .foregroundStyle(AppTheme.sageDark)

                Spacer()

                Button("Next", action: next)
                    .buttonStyle(.bordered)
                    .disabled(page >= totalPages)
            }
            .padding(.vertical, 6)
        }
    }
}
