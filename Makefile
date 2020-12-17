.PHONY: mitmproxy
mitmproxy:
	go build -o mitmproxy cmd/mitmproxy/main.go

.PHONY: clean
clean:
	rm -f mitmproxy

.PHONY: test
test:
	go test ./...
