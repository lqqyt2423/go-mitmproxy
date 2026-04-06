import SwiftUI

@main
struct MITMProxyMacApp: App {
    @StateObject private var proxyManager = ProxyManager()

    var body: some Scene {
        WindowGroup {
            MainView()
                .environmentObject(proxyManager)
        }
        .commands {
            CommandGroup(after: .appInfo) {
                Button("Start Proxy") {
                    startProxy()
                }
                .keyboardShortcut("r", modifiers: .command)
                .disabled(proxyManager.isRunning)

                Button("Stop Proxy") {
                    proxyManager.stop()
                }
                .keyboardShortcut(".", modifiers: .command)
                .disabled(!proxyManager.isRunning)

                Divider()

                Button("Clear Flows") {
                    proxyManager.clearFlows()
                }
                .keyboardShortcut("k", modifiers: .command)
            }
        }

        #if os(macOS)
        Settings {
            SettingsView()
                .environmentObject(proxyManager)
        }
        #endif
    }

    private func startProxy() {
        do {
            try proxyManager.start()
        } catch {
            proxyManager.errorMessage = error.localizedDescription
        }
    }
}
