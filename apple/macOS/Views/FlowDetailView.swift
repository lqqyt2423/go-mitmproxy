import SwiftUI

struct FlowDetailView: View {
    @ObservedObject var flow: FlowItem
    @EnvironmentObject var proxy: ProxyManager
    @State private var selectedTab = 0
    @State private var requestBody: Data?
    @State private var responseBody: Data?

    var body: some View {
        VStack(alignment: .leading, spacing: 0) {
            // Header
            headerView
                .padding()

            Divider()

            // Tabs
            Picker("", selection: $selectedTab) {
                Text("Request").tag(0)
                Text("Response").tag(1)
                if !flow.webSocketMessages.isEmpty {
                    Text("WebSocket (\(flow.webSocketMessages.count))").tag(2)
                }
                if !flow.sseEvents.isEmpty {
                    Text("SSE (\(flow.sseEvents.count))").tag(3)
                }
            }
            .pickerStyle(.segmented)
            .padding(.horizontal)
            .padding(.vertical, 8)

            // Tab content
            ScrollView {
                switch selectedTab {
                case 0: requestView
                case 1: responseView
                case 2: webSocketView
                case 3: sseView
                default: EmptyView()
                }
            }
            .frame(maxWidth: .infinity, maxHeight: .infinity)
        }
    }

    // MARK: - Header

    private var headerView: some View {
        VStack(alignment: .leading, spacing: 4) {
            HStack {
                Text(flow.request.method)
                    .font(.system(.title3, design: .monospaced, weight: .bold))
                if let code = flow.statusCode {
                    Text("\(code)")
                        .font(.system(.title3, design: .monospaced))
                        .foregroundStyle(code < 400 ? .green : .red)
                }
                if let err = flow.errorMessage {
                    Text("ERROR")
                        .font(.caption)
                        .foregroundStyle(.red)
                    Text(err)
                        .font(.caption)
                        .foregroundStyle(.secondary)
                }
            }
            Text(flow.request.url)
                .font(.system(.body, design: .monospaced))
                .foregroundStyle(.secondary)
                .textSelection(.enabled)
        }
    }

    // MARK: - Request tab

    private var requestView: some View {
        VStack(alignment: .leading, spacing: 12) {
            headersSection(title: "Request Headers", headers: flow.request.headers)

            if flow.request.bodyLen > 0 {
                bodySection(title: "Request Body", body: requestBody, bodyLen: flow.request.bodyLen) {
                    requestBody = proxy.getRequestBody(flowID: flow.id)
                }
            }
        }
        .padding()
    }

    // MARK: - Response tab

    private var responseView: some View {
        VStack(alignment: .leading, spacing: 12) {
            if let resp = flow.response {
                headersSection(title: "Response Headers", headers: resp.headers)

                if resp.bodyLen > 0 {
                    bodySection(title: "Response Body", body: responseBody, bodyLen: resp.bodyLen) {
                        responseBody = proxy.getResponseBody(flowID: flow.id)
                    }
                }

                HStack {
                    Text("Duration: \(resp.durationMs)ms")
                    Text("Size: \(resp.bodyLen) bytes")
                }
                .font(.caption)
                .foregroundStyle(.secondary)
            } else {
                Text("Waiting for response...")
                    .foregroundStyle(.secondary)
            }
        }
        .padding()
    }

    // MARK: - WebSocket tab

    private var webSocketView: some View {
        LazyVStack(alignment: .leading, spacing: 4) {
            ForEach(Array(flow.webSocketMessages.enumerated()), id: \.offset) { _, msg in
                HStack(alignment: .top) {
                    Image(systemName: msg.fromClient ? "arrow.up.circle.fill" : "arrow.down.circle.fill")
                        .foregroundStyle(msg.fromClient ? .blue : .green)
                    VStack(alignment: .leading) {
                        Text(msg.direction)
                            .font(.caption)
                            .foregroundStyle(.secondary)
                        Text(msg.content)
                            .font(.system(.body, design: .monospaced))
                            .textSelection(.enabled)
                    }
                }
                .padding(.vertical, 2)
                Divider()
            }
        }
        .padding()
    }

    // MARK: - SSE tab

    private var sseView: some View {
        LazyVStack(alignment: .leading, spacing: 4) {
            ForEach(Array(flow.sseEvents.enumerated()), id: \.offset) { _, event in
                VStack(alignment: .leading, spacing: 2) {
                    HStack {
                        if let type = event.event {
                            Text("event: \(type)")
                                .font(.system(.caption, design: .monospaced, weight: .bold))
                        }
                        if let id = event.id {
                            Text("id: \(id)")
                                .font(.system(.caption, design: .monospaced))
                                .foregroundStyle(.secondary)
                        }
                    }
                    Text(event.data)
                        .font(.system(.body, design: .monospaced))
                        .textSelection(.enabled)
                }
                .padding(.vertical, 4)
                Divider()
            }
        }
        .padding()
    }

    // MARK: - Helpers

    private func headersSection(title: String, headers: [String: String]) -> some View {
        VStack(alignment: .leading, spacing: 4) {
            Text(title)
                .font(.headline)
            ForEach(headers.sorted(by: { $0.key < $1.key }), id: \.key) { key, value in
                HStack(alignment: .top) {
                    Text(key + ":")
                        .font(.system(.body, design: .monospaced, weight: .semibold))
                        .foregroundStyle(.blue)
                    Text(value)
                        .font(.system(.body, design: .monospaced))
                        .textSelection(.enabled)
                }
            }
        }
    }

    @ViewBuilder
    private func bodySection(title: String, body: Data?, bodyLen: Int, load: @escaping () -> Void) -> some View {
        VStack(alignment: .leading, spacing: 4) {
            HStack {
                Text(title)
                    .font(.headline)
                if body == nil {
                    Button("Load (\(bodyLen) bytes)") { load() }
                        .buttonStyle(.bordered)
                        .controlSize(.small)
                }
            }
            if let body = body {
                if let text = String(data: body, encoding: .utf8) {
                    // Try to format as JSON
                    if let json = try? JSONSerialization.jsonObject(with: body),
                       let pretty = try? JSONSerialization.data(withJSONObject: json, options: .prettyPrinted),
                       let prettyStr = String(data: pretty, encoding: .utf8) {
                        Text(prettyStr)
                            .font(.system(.body, design: .monospaced))
                            .textSelection(.enabled)
                    } else {
                        Text(text)
                            .font(.system(.body, design: .monospaced))
                            .textSelection(.enabled)
                    }
                } else {
                    Text("[\(body.count) bytes binary data]")
                        .foregroundStyle(.secondary)
                }
            }
        }
    }
}
