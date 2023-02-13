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

.PHONY: test
test:
	go test ./... -v

.PHONY: dev
dev:
	go run $(shell ls cmd/go-mitmproxy/*.go | grep -v _test.go)
