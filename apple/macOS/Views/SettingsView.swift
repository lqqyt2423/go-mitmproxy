import SwiftUI

struct SettingsView: View {
    @EnvironmentObject var proxy: ProxyManager
    @AppStorage("proxyPort") private var port = 9080
    @AppStorage("sslInsecure") private var sslInsecure = false
    @State private var certStatus = ""

    var body: some View {
        Form {
            Section("Proxy") {
                TextField("Port", value: $port, format: .number)
                    .frame(width: 200)
                Toggle("Skip upstream TLS verification", isOn: $sslInsecure)
            }

            Section("Certificate") {
                HStack {
                    Button("Export CA Certificate...") { exportCert() }
                    #if os(macOS)
                    Button("Install to Keychain") { installToKeychain() }
                    #endif
                }
                if !certStatus.isEmpty {
                    Text(certStatus)
                        .font(.caption)
                        .foregroundStyle(.secondary)
                }
            }

            Section("System Proxy") {
                #if os(macOS)
                Button("Enable System Proxy") { SystemProxy.enable(port: port) }
                Button("Disable System Proxy") { SystemProxy.disable() }
                Text("Configures macOS system HTTP/HTTPS proxy settings")
                    .font(.caption)
                    .foregroundStyle(.secondary)
                #endif
            }
        }
        .formStyle(.grouped)
        .frame(width: 450)
        .padding()
    }

    private func exportCert() {
        #if os(macOS)
        guard proxy.isRunning else {
            certStatus = "Start the proxy first"
            return
        }
        do {
            let pem = try proxy.getCACertPEM()
            let panel = NSSavePanel()
            panel.nameFieldStringValue = "mitmproxy-ca-cert.pem"
            panel.allowedContentTypes = [.init(filenameExtension: "pem")!]
            if panel.runModal() == .OK, let url = panel.url {
                try CertManager.exportPEM(pem, to: url)
                certStatus = "Certificate exported to \(url.lastPathComponent)"
            }
        } catch {
            certStatus = "Export failed: \(error.localizedDescription)"
        }
        #endif
    }

    #if os(macOS)
    private func installToKeychain() {
        guard proxy.isRunning else {
            certStatus = "Start the proxy first"
            return
        }
        do {
            let der = try proxy.getCACertDER()
            if CertManager.addToKeychain(derData: der) {
                certStatus = "Certificate added to keychain. Open Keychain Access to set trust."
            } else {
                certStatus = "Failed to add certificate to keychain"
            }
        } catch {
            certStatus = "Error: \(error.localizedDescription)"
        }
    }
    #endif
}
