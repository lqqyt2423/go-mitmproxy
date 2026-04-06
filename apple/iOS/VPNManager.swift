import Foundation
import NetworkExtension

/// Manages the VPN tunnel that routes traffic through the local MITM proxy.
@MainActor
class VPNManager: ObservableObject {
    @Published var isConnected = false
    @Published var status: NEVPNStatus = .disconnected
    @Published var errorMessage: String?

    private var manager: NETunnelProviderManager?
    private var observer: NSObjectProtocol?

    init() {
        loadManager()
    }

    deinit {
        if let observer = observer {
            NotificationCenter.default.removeObserver(observer)
        }
    }

    // MARK: - Public

    /// Start the VPN tunnel (which starts the packet tunnel extension + Go proxy).
    func connect() {
        Task {
            do {
                let mgr = try await getOrCreateManager()
                try mgr.connection.startVPNTunnel()
            } catch {
                errorMessage = error.localizedDescription
            }
        }
    }

    /// Stop the VPN tunnel.
    func disconnect() {
        manager?.connection.stopVPNTunnel()
    }

    /// Toggle VPN on/off.
    func toggle() {
        if isConnected {
            disconnect()
        } else {
            connect()
        }
    }

    // MARK: - Private

    private func loadManager() {
        NETunnelProviderManager.loadAllFromPreferences { [weak self] managers, error in
            Task { @MainActor in
                if let existing = managers?.first {
                    self?.manager = existing
                    self?.observeStatus(existing)
                    self?.updateStatus(existing.connection.status)
                }
            }
        }
    }

    private func getOrCreateManager() async throws -> NETunnelProviderManager {
        if let mgr = manager {
            return mgr
        }

        let mgr = NETunnelProviderManager()

        let proto = NETunnelProviderProtocol()
        // Bundle ID of the PacketTunnelExtension target
        proto.providerBundleIdentifier = Bundle.main.bundleIdentifier! + ".PacketTunnel"
        proto.serverAddress = "127.0.0.1"
        proto.providerConfiguration = [
            "proxyPort": 9080,
            "webPort": 9081
        ]

        mgr.protocolConfiguration = proto
        mgr.localizedDescription = "go-mitmproxy"
        mgr.isEnabled = true

        try await mgr.saveToPreferences()
        try await mgr.loadFromPreferences()

        manager = mgr
        observeStatus(mgr)
        return mgr
    }

    private func observeStatus(_ mgr: NETunnelProviderManager) {
        if let observer = observer {
            NotificationCenter.default.removeObserver(observer)
        }
        observer = NotificationCenter.default.addObserver(
            forName: .NEVPNStatusDidChange,
            object: mgr.connection,
            queue: .main
        ) { [weak self] _ in
            Task { @MainActor in
                self?.updateStatus(mgr.connection.status)
            }
        }
    }

    private func updateStatus(_ vpnStatus: NEVPNStatus) {
        status = vpnStatus
        isConnected = (vpnStatus == .connected)
    }
}
