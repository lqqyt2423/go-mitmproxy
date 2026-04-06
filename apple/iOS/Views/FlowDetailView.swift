import SwiftUI

struct FlowDetailView: View {
    @ObservedObject var flow: FlowItem
    @EnvironmentObject var proxy: ProxyManager
    @State private var selectedTab = 0
    @State private var requestBody: Data?
    @State private var responseBody: Data?

    var body: some View {
        VStack(spacing: 0) {
            // URL header
            VStack(alignment: .leading, spacing: 4) {
                HStack {
                    Text(flow.request.method)
                        .font(.system(.headline, design: .monospaced))
                    if let code = flow.statusCode {
                        Text("\(code)")
                            .foregroundStyle(code < 400 ? .green : .red)
                    }
                    Spacer()
                    if let resp = flow.response {
                        Text("\(resp.durationMs)ms")
                            .font(.caption)
                            .foregroundStyle(.secondary)
                    }
                }
                Text(flow.request.url)
                    .font(.system(.caption, design: .monospaced))
                    .foregroundStyle(.secondary)
                    .lineLimit(3)
            }
            .padding()

            // Tabs
            Picker("", selection: $selectedTab) {
                Text("Request").tag(0)
                Text("Response").tag(1)
                if !flow.webSocketMessages.isEmpty { Text("WS").tag(2) }
                if !flow.sseEvents.isEmpty { Text("SSE").tag(3) }
            }
            .pickerStyle(.segmented)
            .padding(.horizontal)

            // Content
            List {
                switch selectedTab {
                case 0: requestSection
                case 1: responseSection
                case 2: webSocketSection
                case 3: sseSection
                default: EmptyView()
                }
            }
            .listStyle(.insetGrouped)
        }
    }

    // MARK: - Request

    @ViewBuilder
    private var requestSection: some View {
        Section("Headers") {
            ForEach(flow.request.headers.sorted(by: { $0.key < $1.key }), id: \.key) { key, value in
                VStack(alignment: .leading) {
                    Text(key).font(.system(.caption, design: .monospaced)).fontWeight(.bold)
                    Text(value).font(.system(.caption, design: .monospaced))
                        .foregroundStyle(.secondary)
                }
            }
        }

        if flow.request.bodyLen > 0 {
            Section("Body") {
                if let body = requestBody {
                    bodyText(body)
                } else {
                    Button("Load Body (\(flow.request.bodyLen) bytes)") {
                        requestBody = proxy.getRequestBody(flowID: flow.id)
                    }
                }
            }
        }
    }

    // MARK: - Response

    @ViewBuilder
    private var responseSection: some View {
        if let resp = flow.response {
            Section("Headers") {
                ForEach(resp.headers.sorted(by: { $0.key < $1.key }), id: \.key) { key, value in
                    VStack(alignment: .leading) {
                        Text(key).font(.system(.caption, design: .monospaced)).fontWeight(.bold)
                        Text(value).font(.system(.caption, design: .monospaced))
                            .foregroundStyle(.secondary)
                    }
                }
            }

            if resp.bodyLen > 0 {
                Section("Body") {
                    if let body = responseBody {
                        bodyText(body)
                    } else {
                        Button("Load Body (\(resp.bodyLen) bytes)") {
                            responseBody = proxy.getResponseBody(flowID: flow.id)
                        }
                    }
                }
            }
        } else {
            Section {
                Text("Waiting for response...")
                    .foregroundStyle(.secondary)
            }
        }
    }

    // MARK: - WebSocket

    @ViewBuilder
    private var webSocketSection: some View {
        Section {
            ForEach(Array(flow.webSocketMessages.enumerated()), id: \.offset) { _, msg in
                HStack(alignment: .top) {
                    Image(systemName: msg.fromClient ? "arrow.up" : "arrow.down")
                        .foregroundStyle(msg.fromClient ? .blue : .green)
                        .font(.caption)
                    Text(msg.content)
                        .font(.system(.caption, design: .monospaced))
                        .lineLimit(5)
                }
            }
        }
    }

    // MARK: - SSE

    @ViewBuilder
    private var sseSection: some View {
        Section {
            ForEach(Array(flow.sseEvents.enumerated()), id: \.offset) { _, event in
                VStack(alignment: .leading, spacing: 2) {
                    if let type = event.event {
                        Text("event: \(type)")
                            .font(.system(.caption2, design: .monospaced)).fontWeight(.bold)
                    }
                    Text(event.data)
                        .font(.system(.caption, design: .monospaced))
                        .lineLimit(10)
                }
            }
        }
    }

    // MARK: - Helper

    @ViewBuilder
    private func bodyText(_ data: Data) -> some View {
        if let text = String(data: data, encoding: .utf8) {
            if let json = try? JSONSerialization.jsonObject(with: data),
               let pretty = try? JSONSerialization.data(withJSONObject: json, options: .prettyPrinted),
               let prettyStr = String(data: pretty, encoding: .utf8) {
                Text(prettyStr)
                    .font(.system(.caption, design: .monospaced))
                    .textSelection(.enabled)
            } else {
                Text(text)
                    .font(.system(.caption, design: .monospaced))
                    .textSelection(.enabled)
            }
        } else {
            Text("[\(data.count) bytes binary data]")
                .foregroundStyle(.secondary)
        }
    }
}
