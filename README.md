# go-mitmproxy

[mitmproxy](https://mitmproxy.org/) implemented with golang.

用 Golang 实现的中间人攻击（[Man-in-the-middle](https://en.wikipedia.org/wiki/Man-in-the-middle_attack)），解析、监测、篡改 HTTP/HTTPS 流量。

## 特点

- HTTPS 证书相关逻辑参考 [mitmproxy](https://mitmproxy.org/) 且与之兼容，根证书也保存在 `~/.mitmproxy` 文件夹中，如果之前用过 `mitmproxy` 且根证书已经安装信任，则此 `go-mitmproxy` 可以直接使用
- 支持插件机制，很方便扩展自己需要的功能，可参考 [addon/addon.go](./addon/addon.go)
- 性能优势
    - Golang 天生的性能优势
    - 在进程内存中转发解析 HTTPS 流量，不需通过 tcp端口 或 unix socket 等进程间通信
    - 生成不同域名证书时使用 LRU 缓存，避免重复计算
- 通过环境变量 `SSLKEYLOGFILE` 支持 `Wireshark` 解析分析流量
- 上传/下载大文件时支持流式传输

## 安装

```
go get github.com/lqqyt2423/go-mitmproxy/cmd/mitmproxy
```

## 命令行使用

```
mitmproxy --help

Usage of mitmproxy:
  -addr string
    	proxy listen addr (default ":9080")
  -dump string
    	dump filename
  -dump_level int
    	dump level: 0 - header, 1 - header + body
```

## 作为包引入

参考 [cmd/mitmproxy/main.go](./cmd/mitmproxy/main.go)，可通过自己实现 `AddAddon` 方法添加自己实现的插件。

## TODO

- [x] http handler
- [x] http connect
- [x] cert
- [x] https handler
- [x] logger
- [x] 经内存转发 https 流量
- [x] 忽略某些错误例如：broken pipe, reset by peer, timeout
- [x] websocket 透明转发
- [x] 插件机制
- [x] 命令行参数控制 dump 至文件
- [x] dump level
- [x] 文档
- [ ] support get method with body
- [ ] http2
- [ ] websocket 解析
- [ ] web 界面

## License

[MIT License](./LICENSE)
