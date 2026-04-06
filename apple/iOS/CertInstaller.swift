import Foundation
import UIKit
import Network

/// Handles iOS CA certificate installation flow.
/// Serves a .mobileconfig profile via local HTTP server and opens Safari to install it.
class CertInstaller {

    private var listener: NWListener?
    private var profileData: Data?

    /// Install CA certificate by opening Safari to a local .mobileconfig endpoint.
    /// After installing, user must also go to:
    ///   Settings -> General -> About -> Certificate Trust Settings -> Enable Full Trust
    func install(derData: Data) {
        let profile = CertManager.generateMobileConfig(derData: derData)
        profileData = profile

        // Start a temporary HTTP server to serve the .mobileconfig
        do {
            let params = NWParameters.tcp
            listener = try NWListener(using: params, on: .any)
        } catch {
            return
        }

        listener?.newConnectionHandler = { [weak self] conn in
            self?.handleConnection(conn)
        }

        listener?.stateUpdateHandler = { [weak self] state in
            if case .ready = state {
                guard let port = self?.listener?.port?.rawValue else { return }
                let url = URL(string: "http://localhost:\(port)/cert.mobileconfig")!
                DispatchQueue.main.async {
                    UIApplication.shared.open(url)
                }
                // Auto-stop after 30 seconds
                DispatchQueue.main.asyncAfter(deadline: .now() + 30) {
                    self?.stop()
                }
            }
        }

        listener?.start(queue: .global(qos: .userInitiated))
    }

    func stop() {
        listener?.cancel()
        listener = nil
    }

    private func handleConnection(_ conn: NWConnection) {
        conn.start(queue: .global())
        conn.receive(minimumIncompleteLength: 1, maximumLength: 4096) { [weak self] data, _, _, _ in
            guard let profileData = self?.profileData else {
                conn.cancel()
                return
            }

            let header = """
            HTTP/1.1 200 OK\r\n\
            Content-Type: application/x-apple-aspen-config\r\n\
            Content-Disposition: attachment; filename="mitmproxy-ca.mobileconfig"\r\n\
            Content-Length: \(profileData.count)\r\n\
            Connection: close\r\n\
            \r\n
            """

            var response = header.data(using: .utf8)!
            response.append(profileData)

            conn.send(content: response, completion: .contentProcessed { _ in
                conn.cancel()
                self?.stop()
            })
        }
    }
}
