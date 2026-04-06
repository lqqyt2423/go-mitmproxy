import Foundation

#if os(macOS)
/// Configures macOS system HTTP/HTTPS proxy via networksetup.
enum SystemProxy {

    /// Enable system proxy pointing to localhost on the given port.
    static func enable(port: Int) {
        guard let service = activeNetworkService() else { return }
        run("networksetup", "-setwebproxy", service, "127.0.0.1", "\(port)")
        run("networksetup", "-setsecurewebproxy", service, "127.0.0.1", "\(port)")
        run("networksetup", "-setwebproxystate", service, "on")
        run("networksetup", "-setsecurewebproxystate", service, "on")
    }

    /// Disable system proxy.
    static func disable() {
        guard let service = activeNetworkService() else { return }
        run("networksetup", "-setwebproxystate", service, "off")
        run("networksetup", "-setsecurewebproxystate", service, "off")
    }

    /// Returns the active network service name (e.g. "Wi-Fi").
    private static func activeNetworkService() -> String? {
        let pipe = Pipe()
        let process = Process()
        process.executableURL = URL(fileURLWithPath: "/usr/sbin/networksetup")
        process.arguments = ["-listallnetworkservices"]
        process.standardOutput = pipe
        try? process.run()
        process.waitUntilExit()

        let data = pipe.fileHandleForReading.readDataToEndOfFile()
        guard let output = String(data: data, encoding: .utf8) else { return nil }

        // Skip the first line ("An asterisk (*) denotes...") and find Wi-Fi or first service
        let services = output.components(separatedBy: "\n")
            .dropFirst()
            .map { $0.trimmingCharacters(in: .whitespaces) }
            .filter { !$0.isEmpty && !$0.hasPrefix("*") }

        return services.first(where: { $0 == "Wi-Fi" }) ?? services.first
    }

    @discardableResult
    private static func run(_ args: String...) -> Int32 {
        let process = Process()
        process.executableURL = URL(fileURLWithPath: args[0].hasPrefix("/") ? args[0] : "/usr/sbin/\(args[0])")
        process.arguments = Array(args.dropFirst())
        try? process.run()
        process.waitUntilExit()
        return process.terminationStatus
    }
}
#endif
