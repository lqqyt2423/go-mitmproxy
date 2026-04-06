import SwiftUI

@main
struct MITMProxyiOSApp: App {
    @StateObject private var proxyManager = ProxyManager(maxFlows: 1000)
    @StateObject private var vpnManager = VPNManager()
    @StateObject private var flowReceiver = FlowReceiver()

    var body: some Scene {
        WindowGroup {
            MainView()
                .environmentObject(proxyManager)
                .environmentObject(vpnManager)
                .environmentObject(flowReceiver)
                .onChange(of: vpnManager.isConnected) { connected in
                    if connected {
                        flowReceiver.start(proxyManager: proxyManager)
                    } else {
                        flowReceiver.stop()
                    }
                }
        }
    }
}
