# 测试支持 Node.js 引用 go-mitmproxy

ngmp: Node.js Go MitmProxy

## 安装使用

```bash
npm i ngmp
```

```js
const { createMitmProxy } = require('ngmp');

createMitmProxy()
  .addAddon({
    hookRequestheaders: async (flow) => {
      console.log('in hookRequestheaders', flow);
    },
    hookRequest: async (flow) => {
      console.log('in hookRequest', flow);
    },
    hookResponseheaders: async (flow) => {
      console.log('in hookResponseheaders', flow);
    },
    hookResponse: async (flow) => {
      console.log('in hookResponse', flow);
      flow.response.setBody('hello world');
    },
  })
  .start()
  .registerCloseSignal();
```

可多次调用 addAddon 实现不同功能的逻辑模块，每个 Addon 里面实现 `hookRequestheaders`, `hookRequest`, `hookResponseheaders`, `hookResponse` 这四个方法其中一个或多个即可。

目前所有配置均写死，后续可考虑在 `createMitmProxy`时传入配置参数，用于配置监听的端口号、是否启用 web ui 等功能。

## 开发

思路：c++ 里面循环调用 golang 方法，golang 方法阻塞，直到有数据，然后再 BlockingCall 至 js callback 中

### 编译 golang 代码为静态库

```bash
go build -o libngmp.a -buildmode=c-archive cmd/main.go
```

### 本地编译与测试

```bash
npm i
node example.js
```

### 发布

```bash
npm publish
```

> 目前仅会将本地编译的 libngmp.a 发布至 npm 上面去，所以目前只能 Apple M1 环境下可以测试使用。

## 参考

- [golang cgo](https://pkg.go.dev/cmd/cgo)
- [cgo export struct](https://github.com/golang/go/issues/18412#issuecomment-268847417)
- [node-addon-examples/thread_safe_function_counting](https://github.com/nodejs/node-addon-examples/blob/main/src/6-threadsafe-function/thread_safe_function_counting/node-addon-api/addon.cc)
- [How to publish a n-api module](https://gist.github.com/gabrielschulhof/153edf2819362b8b50758d5ab4ff5e0e)
