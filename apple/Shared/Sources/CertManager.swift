import Foundation
import Security

/// Handles CA certificate export and installation across platforms.
public class CertManager {

    // MARK: - Export

    /// Save PEM certificate to a file.
    public static func exportPEM(_ pem: String, to url: URL) throws {
        try pem.write(to: url, atomically: true, encoding: .utf8)
    }

    /// Save DER certificate to a file.
    public static func exportDER(_ der: Data, to url: URL) throws {
        try der.write(to: url)
    }

    // MARK: - macOS Keychain

    #if os(macOS)
    /// Add CA certificate to the login keychain on macOS.
    /// User must manually set "Always Trust" in Keychain Access afterward.
    public static func addToKeychain(derData: Data) -> Bool {
        guard let cert = SecCertificateCreateWithData(nil, derData as CFData) else {
            return false
        }
        let addQuery: [String: Any] = [
            kSecClass as String: kSecClassCertificate,
            kSecValueRef as String: cert,
            kSecAttrLabel as String: "go-mitmproxy CA"
        ]
        // Remove existing cert with same label first
        SecItemDelete(addQuery as CFDictionary)

        let status = SecItemAdd(addQuery as CFDictionary, nil)
        return status == errSecSuccess || status == errSecDuplicateItem
    }
    #endif

    // MARK: - iOS .mobileconfig

    #if os(iOS)
    /// Generate a .mobileconfig XML profile containing the CA certificate.
    /// User installs this via Safari → Settings → Profile → Install.
    public static func generateMobileConfig(
        derData: Data,
        displayName: String = "go-mitmproxy CA",
        identifier: String = "com.mitmproxy.ca",
        organization: String = "go-mitmproxy"
    ) -> Data {
        let base64Cert = derData.base64EncodedString(options: .lineLength76Characters)
        let payloadUUID = UUID().uuidString
        let profileUUID = UUID().uuidString

        let xml = """
        <?xml version="1.0" encoding="UTF-8"?>
        <!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
        <plist version="1.0">
        <dict>
            <key>PayloadContent</key>
            <array>
                <dict>
                    <key>PayloadCertificateFileName</key>
                    <string>mitmproxy-ca.cer</string>
                    <key>PayloadContent</key>
                    <data>
                    \(base64Cert)
                    </data>
                    <key>PayloadDescription</key>
                    <string>Installs the \(displayName) root certificate</string>
                    <key>PayloadDisplayName</key>
                    <string>\(displayName)</string>
                    <key>PayloadIdentifier</key>
                    <string>\(identifier).cert</string>
                    <key>PayloadType</key>
                    <string>com.apple.security.root</string>
                    <key>PayloadUUID</key>
                    <string>\(payloadUUID)</string>
                    <key>PayloadVersion</key>
                    <integer>1</integer>
                </dict>
            </array>
            <key>PayloadDescription</key>
            <string>Installs \(displayName) for HTTPS traffic inspection</string>
            <key>PayloadDisplayName</key>
            <string>\(displayName)</string>
            <key>PayloadIdentifier</key>
            <string>\(identifier)</string>
            <key>PayloadOrganization</key>
            <string>\(organization)</string>
            <key>PayloadRemovalDisallowed</key>
            <false/>
            <key>PayloadType</key>
            <string>Configuration</string>
            <key>PayloadUUID</key>
            <string>\(profileUUID)</string>
            <key>PayloadVersion</key>
            <integer>1</integer>
        </dict>
        </plist>
        """
        return xml.data(using: .utf8)!
    }
    #endif
}
