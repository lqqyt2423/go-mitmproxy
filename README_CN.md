# go-mitmproxy

[English](./README.md)

`go-mitmproxy` 是一个用 Golang 实现的 [mitmproxy](https://mitmproxy.org/)，支持中间人攻击（Man-in-the-middle）并解析、监测、篡改 HTTP/HTTPS 流量。

## 主要功能

- 解析 HTTP/HTTPS 流量，可通过 [WEB 界面](#web-界面)查看流量详情。
- 支持[插件机制](#通过开发插件添加功能)，方便扩展自己需要的功能。多种事件 HOOK 可参考 [examples](./examples)。
- HTTPS 证书相关逻辑与 [mitmproxy](https://mitmproxy.org/) 兼容，并保存在 `~/.mitmproxy` 文件夹中。如果之前已经用过 `mitmproxy` 并安装信任了根证书，则 `go-mitmproxy` 可以直接使用。
- 支持 Map Remote 和 Map Local。
- 更多功能请参考[配置文档](#更多参数)。

## 暂未实现的功能

- 只支持客户端显示设置代理，不支持透明代理模式。
- 暂不支持 http/2 协议解析和 websocket 协议解析。

> 如需了解显示设置代理和透明代理模式的区别，请参考 Python 版本的 mitmproxy 文档：[How mitmproxy works](https://docs.mitmproxy.org/stable/concepts-howmitmproxyworks/)。`go-mitmproxy` 目前支持文中提到的『Explicit HTTP』和『Explicit HTTPS』。

## 命令行工具

### 安装

```bash
go install github.com/lqqyt2423/go-mitmproxy/cmd/go-mitmproxy@latest
```

### 使用

使用以下命令启动 go-mitmproxy 代理服务器：

```bash
go-mitmproxy
```

启动后，HTTP 代理地址默认为 9080 端口，Web 界面默认在 9081 端口。

首次启动后需安装证书以解析 HTTPS 流量，证书会在首次启动命令后自动生成，路径为 `~/.mitmproxy/mitmproxy-ca-cert.pem`。安装步骤可参考 Python mitmproxy 文档：[About Certificates](https://docs.mitmproxy.org/stable/concepts-certificates/)。

### 更多参数

可以使用以下命令查看 go-mitmproxy 的更多参数：

```bash
go-mitmproxy -h
```

```txt
Usage of go-mitmproxy:
  -addr string
    	代理监听地址 (默认值为 ":9080")
  -allow_hosts []string
    	HTTPS解析域名白名单
  -cert_path string
    	生成证书文件路径
  -debug int
    	调试模式：1-打印调试日志，2-显示调试来源
  -f string
    	从文件名读取配置，传入json配置文件地址
  -ignore_hosts value
    	HTTPS解析域名黑名单
  -map_local string
    	map local json配置文件地址
  -map_remote string
    	map remote json配置文件地址
  -ssl_insecure
    	不验证上游服务器的 SSL/TLS 证书
  -upstream string
    	upstream proxy
  -version
    	显示 go-mitmproxy 版本
  -web_addr string
    	web 界面监听地址 (默认值为 ":9081")
```

## 作为包引入开发功能

### 简单示例

```golang
package main

import (
	"log"

	"github.com/lqqyt2423/go-mitmproxy/proxy"
)

func main() {
	opts := &proxy.Options{
		Addr:              ":9080",
		StreamLargeBodies: 1024 * 1024 * 5,
	}

	p, err := proxy.NewProxy(opts)
	if err != nil {
		log.Fatal(err)
	}

	log.Fatal(p.Start())
}
```

### 通过开发插件添加功能

参考示例 [examples](./examples)，可通过自己实现 `AddAddon` 方法添加自己实现的插件。

下面列出目前支持的事件节点：

```golang
type Addon interface {
	// 一个客户端已经连接到了mitmproxy。请注意，一个连接可能对应多个HTTP请求。
	ClientConnected(*ClientConn)

	// 一个客户端连接已关闭（由我们或客户端关闭）。
	ClientDisconnected(*ClientConn)

	// mitmproxy 已连接到服务器。
	ServerConnected(*ConnContext)

	// 服务器连接已关闭（由我们或服务器关闭）。
	ServerDisconnected(*ConnContext)

	// 与服务器的TLS握手已成功完成。
	TlsEstablishedServer(*ConnContext)

	// HTTP请求头已成功读取。此时，请求体为空。
	Requestheaders(*Flow)

	// 完整的HTTP请求已被读取。
	Request(*Flow)

	// HTTP响应头已成功读取。此时，响应体为空。
	Responseheaders(*Flow)

	// 完整的HTTP响应已被读取。
	Response(*Flow)

	// 流式请求体修改器
	StreamRequestModifier(*Flow, io.Reader) io.Reader

	// 流式响应体修改器
	StreamResponseModifier(*Flow, io.Reader) io.Reader
}
```

## WEB 界面

你可以通过浏览器访问 http://localhost:9081/ 来使用 WEB 界面。

### 功能点

- 查看 HTTP/HTTPS 请求的详细信息
- 支持对 JSON 请求/响应进行格式化预览
- 支持二进制模式查看响应体
- 支持高级的筛选过滤规则
- 支持请求断点功能

### 截图示例

![](./assets/web-1.png)

![](./assets/web-2.png)

![](./assets/web-3.png)

### 赞助我

如果你觉得这个项目对你有帮助，不妨考虑给我买杯咖啡。

赞助时可备注来源 go-mitmproxy，我会将你添加至下面的赞助列表中。

<div align="center">
	<img alt="sponsorme" src="./assets/sponsor-me.jpeg" style="width: 300px" />
</div>

感谢以下赞助者：

暂无

## License

[MIT License](./LICENSE)
