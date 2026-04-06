import SwiftUI

struct MainView: View {
    @EnvironmentObject var proxy: ProxyManager
    @EnvironmentObject var vpn: VPNManager
    @State private var selectedFlow: FlowItem?
    @State private var searchText = ""
    @State private var showSettings = false
    @AppStorage("proxyMode") private var proxyMode: String = "direct" // "direct" or "vpn"

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
        NavigationView {
            VStack(spacing: 0) {
                // Proxy control bar
                proxyBar
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
                NavigationView {
                    FlowDetailView(flow: flow)
                        .navigationTitle("Detail")
                        .navigationBarTitleDisplayMode(.inline)
                        .toolbar {
                            ToolbarItem(placement: .navigationBarTrailing) {
                                Button("Done") { selectedFlow = nil }
                            }
                        }
                }
                .navigationViewStyle(.stack)
            }
            .sheet(isPresented: $showSettings) {
                NavigationView {
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
                .navigationViewStyle(.stack)
            }
        }
        .navigationViewStyle(.stack)
    }

    // MARK: - Proxy Bar (supports both modes)

    private var proxyBar: some View {
        VStack(spacing: 8) {
            // Mode selector
            Picker("Mode", selection: $proxyMode) {
                Text("Direct").tag("direct")
                Text("VPN").tag("vpn")
            }
            .pickerStyle(.segmented)
            .disabled(isActive)

            HStack {
                Circle()
                    .fill(isActive ? .green : .gray)
                    .frame(width: 10, height: 10)

                Text(statusText)
                    .font(.subheadline)

                Spacer()

                Text("\(proxy.flows.count) flows")
                    .font(.caption)
                    .foregroundStyle(.secondary)

                Button(action: { toggleProxy() }) {
                    Text(isActive ? "Stop" : "Start")
                        .font(.subheadline).fontWeight(.bold)
                        .foregroundStyle(.white)
                        .padding(.horizontal, 16)
                        .padding(.vertical, 6)
                        .background(isActive ? Color.red : Color.green)
                        .clipShape(Capsule())
                }
            }

            if proxyMode == "direct" && proxy.isRunning {
                Text("Set WiFi proxy to 127.0.0.1:9080")
                    .font(.caption2)
                    .foregroundStyle(.orange)
            }
        }
    }

    private var isActive: Bool {
        proxyMode == "direct" ? proxy.isRunning : vpn.isConnected
    }

    private var statusText: String {
        if proxyMode == "direct" {
            return proxy.isRunning ? "Proxy Running (:9080)" : "Proxy Stopped"
        }
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

    private func toggleProxy() {
        if proxyMode == "direct" {
            if proxy.isRunning {
                proxy.stop()
            } else {
                do {
                    try proxy.start()
                } catch {
                    proxy.errorMessage = error.localizedDescription
                }
            }
        } else {
            vpn.toggle()
        }
    }
}

// Reuse the same FlowRow from macOS
struct FlowRow: View {
    @ObservedObject var flow: FlowItem

    var body: some View {
        HStack(spacing: 8) {
            Text(flow.request.method)
                .font(.system(.caption, design: .monospaced))
                .fontWeight(.bold)
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
