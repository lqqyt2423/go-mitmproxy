.PHONY: mitmproxy
mitmproxy:
	go build -o mitmproxy cmd/mitmproxy/main.go

.PHONY: testserver
testserver:
	go build -o testserver cmd/testserver/main.go

.PHONY: clean
clean:
	rm -f mitmproxy testserver
