import SwiftUI

struct SettingsView: View {
    @EnvironmentObject var proxy: ProxyManager
    @State private var certInstaller = CertInstaller()
    @State private var showingCertGuide = false

    var body: some View {
        Form {
            Section("Certificate") {
                Button("Install CA Certificate") {
                    installCert()
                }

                Button("Certificate Trust Guide") {
                    showingCertGuide = true
                }
                .foregroundStyle(.blue)
            }

            Section(footer: Text("The VPN tunnel routes HTTP/HTTPS traffic through the local proxy for inspection. No data leaves your device.")) {
                HStack {
                    Text("Proxy Port")
                    Spacer()
                    Text("9080")
                        .foregroundStyle(.secondary)
                }
                HStack {
                    Text("Status")
                    Spacer()
                    Text(proxy.isRunning ? "Running" : "Stopped")
                        .foregroundStyle(proxy.isRunning ? .green : .secondary)
                }
            }
        }
        .alert("Certificate Trust Guide", isPresented: $showingCertGuide) {
            Button("OK") {}
        } message: {
            Text("""
            After installing the certificate profile:

            1. Open Settings
            2. Go to General -> About -> Certificate Trust Settings
            3. Enable full trust for "mitmproxy"

            This allows HTTPS traffic decryption.
            """)
        }
    }

    private func installCert() {
        guard proxy.isRunning else {
            proxy.errorMessage = "Start the VPN first"
            return
        }
        do {
            let der = try proxy.getCACertDER()
            certInstaller.install(derData: der)
        } catch {
            proxy.errorMessage = error.localizedDescription
        }
    }
}
