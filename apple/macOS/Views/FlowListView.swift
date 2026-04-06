import SwiftUI

struct FlowListView: View {
    let flows: [FlowItem]
    @Binding var selectedFlowID: String?

    var body: some View {
        List(flows, selection: $selectedFlowID) { flow in
            FlowRow(flow: flow)
                .tag(flow.id)
        }
        .listStyle(.inset)
    }
}

struct FlowRow: View {
    @ObservedObject var flow: FlowItem

    var body: some View {
        HStack(spacing: 8) {
            // Method badge
            Text(flow.request.method)
                .font(.system(.caption, design: .monospaced, weight: .bold))
                .foregroundStyle(.white)
                .padding(.horizontal, 6)
                .padding(.vertical, 2)
                .background(methodColor)
                .clipShape(RoundedRectangle(cornerRadius: 4))
                .frame(width: 60)

            // Status code
            if let code = flow.statusCode {
                Text("\(code)")
                    .font(.system(.caption, design: .monospaced))
                    .foregroundStyle(statusColor)
                    .frame(width: 32)
            } else if flow.errorMessage != nil {
                Image(systemName: "xmark.circle.fill")
                    .foregroundStyle(.red)
                    .frame(width: 32)
            } else {
                ProgressView()
                    .controlSize(.small)
                    .frame(width: 32)
            }

            // URL
            VStack(alignment: .leading, spacing: 2) {
                Text(flow.request.displayHost)
                    .font(.system(.body, design: .monospaced))
                    .lineLimit(1)
                Text(flow.request.displayPath)
                    .font(.caption)
                    .foregroundStyle(.secondary)
                    .lineLimit(1)
            }

            Spacer()

            // Duration & size
            if let resp = flow.response {
                VStack(alignment: .trailing, spacing: 2) {
                    Text("\(resp.durationMs)ms")
                        .font(.caption)
                        .foregroundStyle(.secondary)
                    Text(formatBytes(resp.bodyLen))
                        .font(.caption)
                        .foregroundStyle(.secondary)
                }
            }
        }
        .padding(.vertical, 2)
    }

    private var methodColor: Color {
        switch flow.request.method {
        case "GET": return .blue
        case "POST": return .green
        case "PUT": return .orange
        case "DELETE": return .red
        case "PATCH": return .purple
        default: return .gray
        }
    }

    private var statusColor: Color {
        switch flow.statusColor {
        case .success: return .green
        case .redirect: return .blue
        case .clientError: return .orange
        case .serverError: return .red
        case .pending: return .secondary
        }
    }

    private func formatBytes(_ bytes: Int) -> String {
        if bytes < 1024 { return "\(bytes) B" }
        if bytes < 1024 * 1024 { return String(format: "%.1f KB", Double(bytes) / 1024) }
        return String(format: "%.1f MB", Double(bytes) / (1024 * 1024))
    }
}
