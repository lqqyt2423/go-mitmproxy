import Foundation

// MARK: - JSON models matching Go mobile/flow_json.go

public struct FlowRequest: Codable, Identifiable, Sendable {
    public let id: String
    public let method: String
    public let url: String
    public let proto: String
    public let headers: [String: String]
    public let bodyLen: Int
    public let time: String

    public var displayHost: String {
        URL(string: url)?.host ?? url
    }

    public var displayPath: String {
        URL(string: url)?.path ?? "/"
    }

    public var timestamp: Date? {
        ISO8601DateFormatter().date(from: time)
    }
}

public struct FlowResponse: Codable, Sendable {
    public let id: String
    public let statusCode: Int
    public let headers: [String: String]
    public let bodyLen: Int
    public let durationMs: Int64

    public var contentType: String {
        headers["Content-Type"] ?? headers["content-type"] ?? ""
    }
}

public struct WebSocketMsg: Codable, Sendable {
    public let type: Int
    public let content: String
    public let fromClient: Bool
    public let timestamp: String

    public var isText: Bool { type == 1 }
    public var direction: String { fromClient ? "C->S" : "S->C" }
}

public struct SSEEvent: Codable, Sendable {
    public let id: String?
    public let event: String?
    public let data: String
    public let timestamp: String
}

// MARK: - UI view model

@MainActor
public class FlowItem: ObservableObject, Identifiable {
    public let id: String
    public let request: FlowRequest
    @Published public var response: FlowResponse?
    @Published public var webSocketMessages: [WebSocketMsg] = []
    @Published public var sseEvents: [SSEEvent] = []
    @Published public var errorMessage: String?

    public init(request: FlowRequest) {
        self.id = request.id
        self.request = request
    }

    public var statusCode: Int? { response?.statusCode }
    public var isComplete: Bool { response != nil || errorMessage != nil }

    public var statusColor: StatusColor {
        guard let code = statusCode else { return .pending }
        switch code {
        case 200..<300: return .success
        case 300..<400: return .redirect
        case 400..<500: return .clientError
        case 500..<600: return .serverError
        default: return .pending
        }
    }

    public enum StatusColor {
        case pending, success, redirect, clientError, serverError
    }
}
