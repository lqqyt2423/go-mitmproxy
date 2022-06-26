# go-mitmproxy

[简体中文](./README_CN.md)

[Mitmproxy](https://mitmproxy.org/) implemented with golang. Intercept HTTP & HTTPS requests and responses and modify them.

## Features

- Intercept HTTP & HTTPS requests and responses and modify them on the fly
- SSL/TLS certificates for interception are generated on the fly
- Certificates logic compatible with [mitmproxy](https://mitmproxy.org/), saved at `~/.mitmproxy`. If you used mitmproxy before and installed certificates, then you can use this go-mitmproxy directly
- Addon mechanism, you can add your functions easily, refer to [examples](./examples)
- Performance advantages
  - Golang's inherent performance advantages
  - Forwarding and parsing HTTPS traffic in process memory without inter-process communication such as tcp port or unix socket
  - Use LRU cache when generating certificates of different domain names to avoid double counting
- Support `Wireshark` to analyze traffic through the environment variable `SSLKEYLOGFILE`
- Support streaming when uploading/downloading large files
- Web interface

## Install

```
go install github.com/lqqyt2423/go-mitmproxy/cmd/go-mitmproxy@latest
```

## Usage

### Startup

```
go-mitmproxy
```

After startup, the HTTP proxy address defaults to port 9080, and the web interface defaults to port 9081.

After the first startup, the SSL/TLS certificate will be automatically generated at `~/.mitmproxy/mitmproxy-ca-cert.pem`. You can refer to this link to install: [About Certificates](https://docs.mitmproxy.org/stable/concepts-certificates/).

### Help

```
Usage of go-mitmproxy:
  -addr string
    	proxy listen addr (default ":9080")
  -cert_path string
    	path of generate cert files
  -debug int
    	debug mode: 1 - print debug log, 2 - show debug from
  -dump string
    	dump filename
  -dump_level int
    	dump level: 0 - header, 1 - header + body
  -mapper_dir string
    	mapper files dirpath
  -ssl_insecure
    	not verify upstream server SSL/TLS certificates.
  -version
    	show version
  -web_addr string
    	web interface listen addr (default ":9081")
```

## Usage as package

Refer to [cmd/go-mitmproxy/main.go](./cmd/go-mitmproxy/main.go), you can add your own addon by call `AddAddon` method.

For more examples, please refer to [examples](./examples)

## Web interface

![](./assets/web-1.png)

![](./assets/web-2.png)

![](./assets/web-3.png)

## TODO

- [ ] Support http2
- [ ] Support parse websocket

## License

[MIT License](./LICENSE)
