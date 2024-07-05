# 测试支持 Node.js 引用 go-mitmproxy

ngmp: Node.js Go MitmProxy

## 安装使用

```bash
npm i ngmp
```

```js
const ngmp = require('ngmp');
// todo
ngmp();
```

## 开发

思路：c++ 里面循环调用 golang 方法，golang 方法阻塞，直到有数据，然后再 NonBlockingCall 至 js callback 中

### 编译 golang 代码为静态库

```bash
go build -o libngmp.a -buildmode=c-archive cmd/main.go
```

### 本地编译与测试

```bash
npm i
npm test
```

### 发布

```bash
npm publish
```

> 目前仅会将本地编译的 libngmp.a 发布至 npm 上面去，所以目前只能 Apple M1 环境下可以测试使用。

## 参考

- [golang cgo](https://pkg.go.dev/cmd/cgo)
- [node-addon-examples/promise_callback_demo.cc](https://github.com/nodejs/node-addon-examples/blob/main/src/6-threadsafe-function/promise-callback-demo/node-addon-api/src/promise_callback_demo.cc)
- [How to publish a n-api module](https://gist.github.com/gabrielschulhof/153edf2819362b8b50758d5ab4ff5e0e)
