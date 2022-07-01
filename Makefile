all: mitmproxy

.PHONY: mitmproxy
mitmproxy:
	go build -o go-mitmproxy cmd/go-mitmproxy/main.go

.PHONY: dummycert
dummycert:
	go build -o dummycert cmd/dummycert/main.go

.PHONY: clean
clean:
	rm -f go-mitmproxy dummycert

.PHONY: test
test:
	go test ./... -v
