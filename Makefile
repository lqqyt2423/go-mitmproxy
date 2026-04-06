all: mitmproxy

.PHONY: mitmproxy
mitmproxy:
	go build -o go-mitmproxy cmd/go-mitmproxy/*.go

.PHONY: dummycert
dummycert:
	go build -o dummycert cmd/dummycert/main.go

.PHONY: clean
clean:
	rm -f go-mitmproxy dummycert

# add -race to check data race
# add -count=1 to disable test cache
.PHONY: test
test:
	go test ./... -v

.PHONY: dev
dev:
	go run $(shell ls cmd/go-mitmproxy/*.go | grep -v _test.go)

# gomobile targets
MOBILE_PKG = github.com/lqqyt2423/go-mitmproxy/mobile
XCFRAMEWORK = apple/GomitmproxyMobile.xcframework

.PHONY: mobile-init
mobile-init:
	go install golang.org/x/mobile/cmd/gomobile@latest
	go install golang.org/x/mobile/cmd/gobind@latest
	gomobile init

.PHONY: mobile-framework
mobile-framework:
	gomobile bind -v -ldflags="-s -w" \
		-target ios,iossimulator,macos \
		-o $(XCFRAMEWORK) \
		$(MOBILE_PKG)

.PHONY: mobile-framework-ios
mobile-framework-ios:
	gomobile bind -v -ldflags="-s -w" \
		-target ios,iossimulator \
		-o $(XCFRAMEWORK) \
		$(MOBILE_PKG)

.PHONY: mobile-framework-mac
mobile-framework-mac:
	gomobile bind -v -ldflags="-s -w" \
		-target macos \
		-o $(XCFRAMEWORK) \
		$(MOBILE_PKG)

.PHONY: xcode-gen
xcode-gen:
	cd apple/macOS && xcodegen generate
	cd apple/iOS && xcodegen generate

.PHONY: mobile-clean
mobile-clean:
	rm -rf $(XCFRAMEWORK)
