import SwiftUI

struct MainView: View {
    @EnvironmentObject var proxy: ProxyManager
    @State private var selectedFlowID: String?
    @State private var searchText = ""

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
        NavigationSplitView {
            VStack(spacing: 0) {
                toolbar
                FlowListView(
                    flows: filteredFlows,
                    selectedFlowID: $selectedFlowID
                )
            }
            .frame(minWidth: 350)
        } detail: {
            if let id = selectedFlowID,
               let flow = proxy.flows.first(where: { $0.id == id }) {
                FlowDetailView(flow: flow)
            } else {
                Text("Select a request to view details")
                    .foregroundStyle(.secondary)
            }
        }
        .searchable(text: $searchText, prompt: "Filter by URL or method")
        .navigationTitle("go-mitmproxy")
        .alert("Error", isPresented: .init(
            get: { proxy.errorMessage != nil },
            set: { if !$0 { proxy.errorMessage = nil } }
        )) {
            Button("OK") { proxy.errorMessage = nil }
        } message: {
            Text(proxy.errorMessage ?? "")
        }
    }

    private var toolbar: some View {
        HStack {
            Button(action: toggleProxy) {
                Image(systemName: proxy.isRunning ? "stop.fill" : "play.fill")
            }
            .help(proxy.isRunning ? "Stop Proxy" : "Start Proxy")

            Circle()
                .fill(proxy.isRunning ? .green : .gray)
                .frame(width: 8, height: 8)

            Text(proxy.state)
                .font(.caption)
                .foregroundStyle(.secondary)

            Spacer()

            Text("\(proxy.flows.count) flows")
                .font(.caption)
                .foregroundStyle(.secondary)

            Button(action: { proxy.clearFlows() }) {
                Image(systemName: "trash")
            }
            .help("Clear Flows")
        }
        .padding(.horizontal, 12)
        .padding(.vertical, 8)
    }

    private func toggleProxy() {
        if proxy.isRunning {
            proxy.stop()
        } else {
            do {
                try proxy.start()
            } catch {
                proxy.errorMessage = error.localizedDescription
            }
        }
    }
}
