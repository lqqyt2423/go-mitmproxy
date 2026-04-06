import NetworkExtension
import GomitmproxyMobile

class PacketTunnelProvider: NEPacketTunnelProvider {

    private var engine: MobileEngine?

    override func startTunnel(
        options: [String: NSObject]?,
        completionHandler: @escaping (Error?) -> Void
    ) {
        let proxyPort = (protocolConfiguration as? NETunnelProviderProtocol)?
            .providerConfiguration?["proxyPort"] as? Int ?? 9080

        // Configure tunnel network settings
        let settings = NEPacketTunnelNetworkSettings(tunnelRemoteAddress: "10.0.0.1")

        // IPv4 — route all traffic through tunnel
        let ipv4 = NEIPv4Settings(addresses: ["10.0.0.2"], subnetMasks: ["255.255.255.0"])
        ipv4.includedRoutes = [NEIPv4Route.default()]
        settings.ipv4Settings = ipv4

        // DNS
        settings.dnsSettings = NEDNSSettings(servers: ["8.8.8.8", "1.1.1.1"])

        // Proxy settings — route HTTP/HTTPS to our local Go proxy
        let proxySettings = NEProxySettings()
        proxySettings.httpEnabled = true
        proxySettings.httpServer = NEProxyServer(address: "127.0.0.1", port: proxyPort)
        proxySettings.httpsEnabled = true
        proxySettings.httpsServer = NEProxyServer(address: "127.0.0.1", port: proxyPort)
        proxySettings.matchDomains = [""] // match all domains
        // Exclude localhost from proxy to prevent loops
        proxySettings.exceptionList = ["127.0.0.1", "localhost", "*.local"]
        settings.proxySettings = proxySettings

        setTunnelNetworkSettings(settings) { [weak self] error in
            if let error = error {
                completionHandler(error)
                return
            }

            // Start Go proxy engine
            self?.startGoProxy(port: proxyPort)

            // Begin reading packets (required by NEPacketTunnelProvider)
            self?.readPackets()

            completionHandler(nil)
        }
    }

    override func stopTunnel(
        with reason: NEProviderStopReason,
        completionHandler: @escaping () -> Void
    ) {
        do {
            try engine?.stop()
        } catch {
            NSLog("go-mitmproxy: stop error: \(error)")
        }
        engine = nil
        completionHandler()
    }

    // MARK: - Go Proxy

    private func startGoProxy(port: Int) {
        let handler = ExtensionEventHandler()

        var error: NSError?
        // Empty certPath = in-memory CA (no filesystem access needed)
        let eng = MobileNewEngine("127.0.0.1:\(port)", "", handler, &error)

        if let error = error {
            NSLog("go-mitmproxy: init error: \(error)")
            return
        }

        guard let eng = eng else { return }

        // Memory-conscious settings for Network Extension (~15MB limit)
        eng.setStreamLargeBodies(256 * 1024)  // 256KB
        eng.setFlowStoreLimit(100)

        // Enable WebSocket IPC so the main app can receive flow data via FlowReceiver
        let webPort = (protocolConfiguration as? NETunnelProviderProtocol)?
            .providerConfiguration?["webPort"] as? Int ?? 9081
        eng.setWebAddr("127.0.0.1:\(webPort)")

        do {
            try eng.start()
            engine = eng
            NSLog("go-mitmproxy: proxy started on port \(port)")
        } catch {
            NSLog("go-mitmproxy: start error: \(error)")
        }
    }

    // MARK: - Packet handling

    private func readPackets() {
        packetFlow.readPackets { [weak self] packets, protocols in
            // For HTTP proxy mode, non-HTTP packets pass through the tunnel.
            // The proxy settings handle HTTP/HTTPS routing.
            self?.packetFlow.writePackets(packets, withProtocols: protocols)
            self?.readPackets()
        }
    }
}

// MARK: - Extension Event Handler

/// Minimal event handler for the Network Extension process.
/// Flow data is forwarded to the main app via the WebSocket IPC channel
/// (web.WebAddon running on port 9081 inside the extension).
private class ExtensionEventHandler: NSObject, MobileEventHandlerProtocol {
    func onFlowRequest(_ flowJSON: String?) {
        // Logging only in extension; UI display handled by main app via WebSocket
    }

    func onFlowResponse(_ flowJSON: String?) {}
    func onFlowError(_ flowID: String?, errMsg: String?) {
        if let err = errMsg {
            NSLog("go-mitmproxy: flow error: \(err)")
        }
    }

    func onWebSocketMessage(_ flowID: String?, msgJSON: String?) {}
    func onSSEEvent(_ flowID: String?, eventJSON: String?) {}

    func onStateChanged(_ state: String?, message: String?) {
        NSLog("go-mitmproxy: state -> \(state ?? "unknown"): \(message ?? "")")
    }
}
