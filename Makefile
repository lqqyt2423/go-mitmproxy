all: mitmproxy dummycert

.PHONY: mitmproxy
mitmproxy:
	go build -o mitmproxy cmd/mitmproxy/main.go

.PHONY: dummycert
dummycert:
	go build -o dummycert cmd/dummycert/main.go

.PHONY: clean
clean:
	rm -f mitmproxy dummycert

.PHONY: test
test:
	go test ./...
