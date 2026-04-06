// swift-tools-version: 5.9
import PackageDescription

let package = Package(
    name: "MITMProxyKit",
    platforms: [
        .macOS(.v13),
        .iOS(.v15)
    ],
    products: [
        .library(name: "MITMProxyKit", targets: ["MITMProxyKit"])
    ],
    targets: [
        .target(
            name: "MITMProxyKit",
            path: "Sources"
        )
    ]
)
