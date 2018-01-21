# socks2http

这是一个使用`golang`编写的小程序，用来将`socks5`代理转换成`http/https`代理。

## 安装方法

``` shell
  go get github.com/Yee2/socks2http
```

## 使用说明
软件默认监听本地8080端口，并使用`127.0.0.1:1080`作为远程代理地址，推荐使用[shadowsocks](https://shadowsocks.org/en/index.html)。
```shell
  socks2http -local :8080 -remote 127.0.0.1:1080
```

## 设置代理
大多数软件都支持从命令行读取代理参数，通过下面命令可以设置代理，测试通过`go get`。
```shell
  export http_proxy=127.0.0.1:8080
  export https_proxy=127.0.0.1:8080
```
