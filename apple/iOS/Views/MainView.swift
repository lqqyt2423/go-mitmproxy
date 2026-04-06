import SwiftUI

struct MainView: View {
    @EnvironmentObject var proxy: ProxyManager
    @EnvironmentObject var vpn: VPNManager
    @State private var selectedFlow: FlowItem?
    @State private var searchText = ""
    @State private var showSettings = false

    var filteredFlows: [FlowItem] {
        if searchText.isEmpty {
            return proxy.flows
        }
        return proxy.flows.filter { flow in
            flow.request.url.localizedCaseInsensitiveContains(searchText) ||
            flow.request.method.localizedCaseInsensitiveContains(searchText)
        }
    }

    var body: some View {
        NavigationStack {
            VStack(spacing: 0) {
                // VPN toggle bar
                vpnBar
                    .padding(.horizontal)
                    .padding(.vertical, 8)

                Divider()

                // Flow list
                List(filteredFlows) { flow in
                    FlowRow(flow: flow)
                        .onTapGesture { selectedFlow = flow }
                }
                .listStyle(.plain)
                .searchable(text: $searchText, prompt: "Filter")
            }
            .navigationTitle("go-mitmproxy")
            .toolbar {
                ToolbarItem(placement: .navigationBarLeading) {
                    Button(action: { proxy.clearFlows() }) {
                        Image(systemName: "trash")
                    }
                }
                ToolbarItem(placement: .navigationBarTrailing) {
                    Button(action: { showSettings = true }) {
                        Image(systemName: "gear")
                    }
                }
            }
            .sheet(item: $selectedFlow) { flow in
                NavigationStack {
                    FlowDetailView(flow: flow)
                        .navigationTitle("Detail")
                        .navigationBarTitleDisplayMode(.inline)
                        .toolbar {
                            ToolbarItem(placement: .navigationBarTrailing) {
                                Button("Done") { selectedFlow = nil }
                            }
                        }
                }
            }
            .sheet(isPresented: $showSettings) {
                NavigationStack {
                    SettingsView()
                        .environmentObject(proxy)
                        .navigationTitle("Settings")
                        .navigationBarTitleDisplayMode(.inline)
                        .toolbar {
                            ToolbarItem(placement: .navigationBarTrailing) {
                                Button("Done") { showSettings = false }
                            }
                        }
                }
            }
        }
    }

    private var vpnBar: some View {
        HStack {
            Circle()
                .fill(vpn.isConnected ? .green : .gray)
                .frame(width: 10, height: 10)

            Text(vpnStatusText)
                .font(.subheadline)

            Spacer()

            Text("\(proxy.flows.count) flows")
                .font(.caption)
                .foregroundStyle(.secondary)

            Button(action: { vpn.toggle() }) {
                Text(vpn.isConnected ? "Stop" : "Start")
                    .font(.subheadline.bold())
                    .foregroundStyle(.white)
                    .padding(.horizontal, 16)
                    .padding(.vertical, 6)
                    .background(vpn.isConnected ? Color.red : Color.green)
                    .clipShape(Capsule())
            }
        }
    }

    private var vpnStatusText: String {
        switch vpn.status {
        case .connected: return "VPN Connected"
        case .connecting: return "Connecting..."
        case .disconnecting: return "Disconnecting..."
        case .disconnected: return "VPN Disconnected"
        case .reasserting: return "Reconnecting..."
        case .invalid: return "VPN Not Configured"
        @unknown default: return "Unknown"
        }
    }
}

// Reuse the same FlowRow from macOS
struct FlowRow: View {
    @ObservedObject var flow: FlowItem

    var body: some View {
        HStack(spacing: 8) {
            Text(flow.request.method)
                .font(.system(.caption, design: .monospaced, weight: .bold))
                .foregroundStyle(.white)
                .padding(.horizontal, 6)
                .padding(.vertical, 2)
                .background(methodColor)
                .clipShape(RoundedRectangle(cornerRadius: 4))

            if let code = flow.statusCode {
                Text("\(code)")
                    .font(.system(.caption, design: .monospaced))
                    .foregroundStyle(code < 400 ? .green : .red)
            } else if flow.errorMessage != nil {
                Image(systemName: "xmark.circle.fill")
                    .foregroundStyle(.red)
                    .font(.caption)
            }

            VStack(alignment: .leading, spacing: 1) {
                Text(flow.request.displayHost)
                    .font(.system(.subheadline, design: .monospaced))
                    .lineLimit(1)
                Text(flow.request.displayPath)
                    .font(.caption2)
                    .foregroundStyle(.secondary)
                    .lineLimit(1)
            }

            Spacer()

            if let resp = flow.response {
                Text("\(resp.durationMs)ms")
                    .font(.caption2)
                    .foregroundStyle(.secondary)
            }
        }
    }

    private var methodColor: Color {
        switch flow.request.method {
        case "GET": return .blue
        case "POST": return .green
        case "PUT": return .orange
        case "DELETE": return .red
        default: return .gray
        }
    }
}

extension FlowItem: @retroactive Hashable {
    public static func == (lhs: FlowItem, rhs: FlowItem) -> Bool { lhs.id == rhs.id }
    public nonisolated func hash(into hasher: inout Hasher) { hasher.combine(id) }
}
