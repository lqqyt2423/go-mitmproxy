import Foundation
import GomitmproxyMobile

/// ProxyManager wraps the Go mobile.Engine for SwiftUI consumption.
/// Both macOS and iOS apps use this as the core controller.
@MainActor
public class ProxyManager: ObservableObject {
    @Published public var isRunning = false
    @Published public var state: String = "stopped"
    @Published public var flows: [FlowItem] = []
    @Published public var errorMessage: String?

    private var engine: MobileEngine?
    private let maxFlows: Int

    /// - Parameter maxFlows: Maximum flows kept in the UI list (oldest evicted first).
    public init(maxFlows: Int = 5000) {
        self.maxFlows = maxFlows
    }

    // MARK: - Lifecycle

    /// Start the proxy engine.
    /// - Parameters:
    ///   - port: TCP port to listen on (default 9080)
    ///   - certPath: Certificate storage path. Empty string = in-memory CA.
    ///   - sslInsecure: Skip upstream TLS verification.
    ///   - streamLargeBodies: Threshold for streaming large bodies (bytes).
    public func start(
        port: Int = 9080,
        certPath: String = "",
        sslInsecure: Bool = false,
        streamLargeBodies: Int64 = 5 * 1024 * 1024
    ) throws {
        guard !isRunning else { return }

        let handler = EventBridge(manager: self)
        guard let eng = MobileEngine("127.0.0.1:\(port)", certPath: certPath, handler: handler) else {
            throw ProxyError.initFailed
        }

        eng.setSslInsecure(sslInsecure)
        eng.setStreamLargeBodies(streamLargeBodies)

        engine = eng
        try eng.start()
    }

    /// Stop the proxy engine gracefully.
    public func stop() {
        guard let eng = engine else { return }
        do {
            try eng.stop()
        } catch {
            errorMessage = error.localizedDescription
        }
        engine = nil
    }

    /// Clear all captured flows.
    public func clearFlows() {
        flows.removeAll()
    }

    // MARK: - Certificate

    /// Returns the CA certificate in PEM format.
    public func getCACertPEM() throws -> String {
        guard let eng = engine else { throw ProxyError.notRunning }
        var err: NSError?
        let pem = eng.getCACertPEM(&err)
        if let err = err { throw err }
        return pem
    }

    /// Returns the CA certificate in DER format (raw bytes).
    public func getCACertDER() throws -> Data {
        guard let eng = engine else { throw ProxyError.notRunning }
        // gomobile bridges _Nullable return + NSError** as throws
        let der = try eng.getCACertDER()
        return der
    }

    // MARK: - Body retrieval

    /// Fetch the request body for a specific flow.
    public func getRequestBody(flowID: String) -> Data? {
        guard let eng = engine else { return nil }
        return try? eng.getFlowRequestBody(flowID)
    }

    /// Fetch the response body for a specific flow.
    public func getResponseBody(flowID: String) -> Data? {
        guard let eng = engine else { return nil }
        return try? eng.getFlowResponseBody(flowID)
    }

    // MARK: - Internal

    func handleFlowRequest(_ req: FlowRequest) {
        let item = FlowItem(request: req)
        flows.insert(item, at: 0)
        if flows.count > maxFlows {
            flows.removeLast(flows.count - maxFlows)
        }
    }

    func handleFlowResponse(_ resp: FlowResponse) {
        if let item = flows.first(where: { $0.id == resp.id }) {
            item.response = resp
        }
    }

    func handleFlowError(_ flowID: String, _ errMsg: String) {
        if let item = flows.first(where: { $0.id == flowID }) {
            item.errorMessage = errMsg
        }
    }

    func handleWebSocketMessage(_ flowID: String, _ msg: WebSocketMsg) {
        if let item = flows.first(where: { $0.id == flowID }) {
            item.webSocketMessages.append(msg)
        }
    }

    func handleSSEEvent(_ flowID: String, _ event: SSEEvent) {
        if let item = flows.first(where: { $0.id == flowID }) {
            item.sseEvents.append(event)
        }
    }

    func handleStateChanged(_ newState: String, _ message: String) {
        state = newState
        isRunning = (newState == "running")
        if newState == "error" {
            errorMessage = message
        }
    }

    private func callEngine(_ block: (MobileEngine) throws -> Void) throws {
        guard let eng = engine else { throw ProxyError.notRunning }
        try block(eng)
    }
}

// MARK: - Error types

public enum ProxyError: LocalizedError {
    case initFailed
    case notRunning
    case certExportFailed

    public var errorDescription: String? {
        switch self {
        case .initFailed: return "Failed to initialize proxy engine"
        case .notRunning: return "Proxy engine is not running"
        case .certExportFailed: return "Failed to export CA certificate"
        }
    }
}

// MARK: - Event bridge (Go -> Swift)

/// Implements the Go MobileEventHandlerProtocol, bridging callbacks to ProxyManager.
private class EventBridge: NSObject, MobileEventHandlerProtocol {
    private weak var manager: ProxyManager?
    private let decoder = JSONDecoder()

    init(manager: ProxyManager) {
        self.manager = manager
    }

    func onFlowRequest(_ flowJSON: String?) {
        guard let json = flowJSON,
              let data = json.data(using: .utf8),
              let req = try? decoder.decode(FlowRequest.self, from: data) else { return }
        Task { @MainActor in
            manager?.handleFlowRequest(req)
        }
    }

    func onFlowResponse(_ flowJSON: String?) {
        guard let json = flowJSON,
              let data = json.data(using: .utf8),
              let resp = try? decoder.decode(FlowResponse.self, from: data) else { return }
        Task { @MainActor in
            manager?.handleFlowResponse(resp)
        }
    }

    func onFlowError(_ flowID: String?, errMsg: String?) {
        guard let flowID = flowID, let errMsg = errMsg else { return }
        Task { @MainActor in
            manager?.handleFlowError(flowID, errMsg)
        }
    }

    func onWebSocketMessage(_ flowID: String?, msgJSON: String?) {
        guard let flowID = flowID,
              let json = msgJSON,
              let data = json.data(using: .utf8),
              let msg = try? decoder.decode(WebSocketMsg.self, from: data) else { return }
        Task { @MainActor in
            manager?.handleWebSocketMessage(flowID, msg)
        }
    }

    func onSSEEvent(_ flowID: String?, eventJSON: String?) {
        guard let flowID = flowID,
              let json = eventJSON,
              let data = json.data(using: .utf8),
              let event = try? decoder.decode(SSEEvent.self, from: data) else { return }
        Task { @MainActor in
            manager?.handleSSEEvent(flowID, event)
        }
    }

    func onStateChanged(_ state: String?, message: String?) {
        let s = state ?? "unknown"
        let m = message ?? ""
        Task { @MainActor in
            manager?.handleStateChanged(s, m)
        }
    }
}
