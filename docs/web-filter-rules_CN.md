# 过滤规则使用说明

网页端筛选 Flow 的规则说明文档。

## 基本过滤示例

```
github
```

说明：过滤 URL 中包含 `github` 的 Flow。

## 关键词前可以添加指定作用域

| 作用域    | 说明                                    |
| --------- | --------------------------------------- |
| url       | 请求 URL                                |
| method    | HTTP 请求方法（GET / POST 等）          |
| code      | HTTP 响应状态码                         |
| reqheader | 请求头                                  |
| resheader | 响应头                                  |
| header    | 请求头或响应头                          |
| reqbody   | 请求体                                  |
| resbody   | 响应体                                  |
| body      | 请求体或响应体                          |
| all       | URL / Method / Header / Body 中任意一个 |

## 带作用域的过滤规则示例

### URL 过滤

```
url:github
```

说明：过滤 URL 中包含 `github` 的 Flow。默认作用域即为 URL。

### 请求方法过滤

```
method:get
```

说明：过滤 GET 请求，大小写不敏感。

### 响应状态码过滤

```
code:404
```

说明：过滤响应状态码为 404 的 Flow。

### Header 过滤

```
header:application/json
```

说明：过滤请求头或响应头中包含 `application/json` 的 Flow。

### 请求体/响应体过滤

```
reqbody:token
resbody:token
body:token
```

说明：分别过滤请求体、响应体或请求体/响应体中包含 `token` 的 Flow。

### 全部过滤

```
all:token
```

说明：过滤 URL / Header / Body 中任意一个包含 `token` 的 Flow。

### 过滤字符中带空格

```
resbody:"hello world"
```

说明：过滤响应体中包含 `hello world` 的 Flow。

## 逻辑运算符

### or

```
google or baidu
```

说明：过滤 URL 中包含 `google` 或 `baidu` 的 Flow。

### and

```
method:post and body:hello
```

说明：过滤 POST 请求且请求体或响应体中包含 `hello` 的 Flow。

### not

```
not url:github
```

说明：过滤 URL 中不包含 `github` 的 Flow。

### 组合使用

```
method:get and (url:google or url:baidu) and not resheader:html
```

说明：过滤 GET 请求且 URL 中包含 `google` 或 `baidu` 且响应头中不包含 `html` 的 Flow。
