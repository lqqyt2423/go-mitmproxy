import Foundation

/// Connects to the Go proxy's WebAddon WebSocket endpoint (running in the Network Extension)
/// to receive real-time flow data for display in the main app.
///
/// This reuses the existing web/ binary message protocol:
///   version (1 byte) + type (1 byte) + id (36 bytes) + waitIntercept (1 byte) + content (remaining)
///
/// Message types:
///   0 = conn, 1 = request, 2 = requestBody, 3 = response, 4 = responseBody,
///   5 = connClose, 6 = wsStart, 7 = wsMessage, 8 = wsEnd,
///   30 = sseStart, 31 = sseMessage, 32 = sseEnd
@MainActor
class FlowReceiver: ObservableObject {
    @Published var isConnected = false

    private var wsTask: URLSessionWebSocketTask?
    private var session: URLSession?
    private weak var proxyManager: ProxyManager?
    private var reconnectTimer: Timer?
    private let port: Int

    init(port: Int = 9081) {
        self.port = port
    }

    func start(proxyManager: ProxyManager) {
        self.proxyManager = proxyManager
        connect()
    }

    func stop() {
        reconnectTimer?.invalidate()
        reconnectTimer = nil
        wsTask?.cancel(with: .goingAway, reason: nil)
        wsTask = nil
        isConnected = false
    }

    // MARK: - Connection

    private func connect() {
        let url = URL(string: "ws://127.0.0.1:\(port)/echo")!
        session = URLSession(configuration: .default)
        wsTask = session?.webSocketTask(with: url)
        wsTask?.resume()
        isConnected = true
        receiveMessage()
    }

    private func receiveMessage() {
        wsTask?.receive { [weak self] result in
            Task { @MainActor in
                switch result {
                case .success(let message):
                    self?.handleMessage(message)
                    self?.receiveMessage()
                case .failure:
                    self?.isConnected = false
                    self?.scheduleReconnect()
                }
            }
        }
    }

    private func scheduleReconnect() {
        reconnectTimer?.invalidate()
        reconnectTimer = Timer.scheduledTimer(withTimeInterval: 2.0, repeats: false) { [weak self] _ in
            Task { @MainActor in
                self?.connect()
            }
        }
    }

    // MARK: - Message parsing

    private func handleMessage(_ message: URLSessionWebSocketTask.Message) {
        switch message {
        case .data(let data):
            parseWebMessage(data)
        case .string(let text):
            if let data = text.data(using: .utf8) {
                parseWebMessage(data)
            }
        @unknown default:
            break
        }
    }

    /// Parse the binary message format from web/message.go
    private func parseWebMessage(_ data: Data) {
        guard data.count >= 40 else { return } // version(1) + type(1) + id(36) + waitIntercept(1) + min content

        let version = data[0]
        guard version == 2 else { return } // messageVersion = 2

        let msgType = data[1]
        let idString = String(data: data[2..<38], encoding: .utf8) ?? ""
        // let waitIntercept = data[38]  // not used in display
        let content = data.count > 39 ? data[39...] : Data()

        switch msgType {
        case 1: // messageTypeRequest
            parseRequest(id: idString, content: Data(content))
        case 3: // messageTypeResponse
            parseResponse(id: idString, content: Data(content))
        case 7: // messageTypeWebSocketMessage
            parseWSMessage(id: idString, content: Data(content))
        case 31: // messageTypeSSEMessage
            parseSSEMessage(id: idString, content: Data(content))
        default:
            break // conn, body, close, start/end events — skip for now
        }
    }

    private func parseRequest(id: String, content: Data) {
        // content is JSON: {"request": {...}, "connId": "..."}
        guard let json = try? JSONSerialization.jsonObject(with: content) as? [String: Any],
              let reqDict = json["request"] as? [String: Any],
              let method = reqDict["method"] as? String,
              let urlStr = reqDict["url"] as? String else { return }

        let headers = (reqDict["header"] as? [String: [String]])?.compactMapValues { $0.first } ?? [:]
        let req = FlowRequest(
            id: id,
            method: method,
            url: urlStr,
            proto: reqDict["proto"] as? String ?? "",
            headers: headers,
            bodyLen: 0,
            time: ISO8601DateFormatter().string(from: Date())
        )
        proxyManager?.handleFlowRequest(req)
    }

    private func parseResponse(id: String, content: Data) {
        guard let json = try? JSONSerialization.jsonObject(with: content) as? [String: Any],
              let statusCode = json["statusCode"] as? Int else { return }

        let headers = (json["header"] as? [String: [String]])?.compactMapValues { $0.first } ?? [:]
        let resp = FlowResponse(
            id: id,
            statusCode: statusCode,
            headers: headers,
            bodyLen: 0,
            durationMs: 0
        )
        proxyManager?.handleFlowResponse(resp)
    }

    private func parseWSMessage(id: String, content: Data) {
        guard let json = try? JSONSerialization.jsonObject(with: content) as? [String: Any],
              let msgDict = json["message"] as? [String: Any] else { return }

        let msg = WebSocketMsg(
            type: msgDict["type"] as? Int ?? 1,
            content: msgDict["content"] as? String ?? "",
            fromClient: msgDict["fromClient"] as? Bool ?? true,
            timestamp: ISO8601DateFormatter().string(from: Date())
        )
        proxyManager?.handleWebSocketMessage(id, msg)
    }

    private func parseSSEMessage(id: String, content: Data) {
        guard let json = try? JSONSerialization.jsonObject(with: content) as? [String: Any],
              let eventDict = json["event"] as? [String: Any] else { return }

        let event = SSEEvent(
            id: eventDict["id"] as? String,
            event: eventDict["event"] as? String,
            data: eventDict["data"] as? String ?? "",
            timestamp: ISO8601DateFormatter().string(from: Date())
        )
        proxyManager?.handleSSEEvent(id, event)
    }
}
