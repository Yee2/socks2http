package main

import (
	"net"
	"log"
	"fmt"
	"bytes"
	"errors"
	"encoding/binary"
	"strconv"
	"io"
	"bufio"
	"flag"
	"net/url"
	"strings"
	"time"
	"runtime"
	"path/filepath"
)
var (
	local,remote string
	direct bool
	timeout = time.Second * 10
	)
func main(){
	flag.StringVar(&local,"local", ":8080", "监听本地端口")
	flag.BoolVar(&direct,"direct", false, "直接连接 不通过代理")
	flag.StringVar(&remote,"remote", "127.0.0.1:1080", "socks5代理服务器地址（默认：127.0.0.1:1080）")
	flag.Parse()
	if local == "" || remote ==""{
		flag.Usage()
		return
	}
	l, err := net.Listen("tcp", local)
	if err != nil {
		log.Fatalf("监听失败:%s",err)
	}
	log.Printf("开始监听 %s ，代理地址 %s\n", local,remote)
	for{
		client,err := l.Accept()
		checkErr(err)
		go handle(client)
	}
}

func checkErr (err error){
	if err != nil{
		_, file, line, ok := runtime.Caller(1)
		if ok{
			panic(fmt.Errorf("%s:%d: %s",filepath.Base(file),line,err))
		}
		panic(err)
	}
}

func handle(Source net.Conn){
	defer func() {
		if e := recover(); e != nil{
			log.Printf("%s",e)
		}
	}()
	if Source == nil{
		return
	}
	defer Source.Close()

	var method,host,protocol string
	requestBuffer := bytes.NewBuffer([]byte{})


	r := bufio.NewReader(Source)
	line,err := readLineSlice(r)

	fmt.Sscanf(string(line), "%s%s%s", &method, &host,&protocol)
	requestBuffer.Write(line)
	requestBuffer.WriteByte('\n')
	// 读取完整的请求信息
	for {
		line,err = readLineSlice(r)
		if err == io.EOF{
			break
		}
		checkErr(err)
		if len(line) == 0{
			// 读取请求头 完成
			break
		}
		requestBuffer.Write(line)
		requestBuffer.WriteByte('\n')
	}
	requestBuffer.WriteByte('\n')
	//log.Printf("请求头信息:\n%s--------------",requestBuffer.Bytes())
	var Destination net.Conn
	if direct{
		Destination,err = doDirect(host)
	}else{
		Destination,err = socks5(host)
	}
	checkErr(err)
	defer Destination.Close()
	// 连接成功，开始转发。

	if method == "CONNECT"{
		_,err := Source.Write([]byte("HTTP/1.1 200 OK\n\n"))
		checkErr(err)
	}else{
		_,err = Destination.Write(requestBuffer.Bytes())
		checkErr(err)
	}
	go io.Copy(Destination,Source)
	io.Copy(Source,Destination)

	// 停止转发
}

// 通过socks5转发
func socks5(host string) (Conn net.Conn,err error){
	defer func() {
		if e := recover(); e != nil{
			err = fmt.Errorf("socks5 proxy error:%s",e)
			if Conn != nil{
				Conn.Close()
				Conn = nil
			}
		}
	}()
	var (
		addr,port string
	)
	addr,port,err = net.SplitHostPort(address(host))
	checkErr(err)
	Conn,err = net.DialTimeout("tcp",remote,timeout)
	checkErr(err)

	Conn.Write([]byte{0x05,01,00})
	var raw [1024]byte
	n,err := Conn.Read(raw[:])
	checkErr(err)
	if binary.BigEndian.Uint16(raw[:2]) != 0x0500{
		panic(errors.New("不支持该代理服务器！"))
	}

	// 将端口转为两个字节 二进制表示

	p := make([]byte,2,2)
	i,err := strconv.Atoi(port)
	binary.BigEndian.PutUint16(p, uint16(i))

	if i := net.ParseIP(addr); i != nil{
		// TODO : 添加IPv6支持
		// IP地址
		if ipv4 := i.To4(); ipv4 != nil{
			_,err = Conn.Write(append(append([]byte{0x05,0x01,0x00,0x01},[]byte(ipv4)...),p...))
			checkErr(err)
		}else if ipv6 := i.To16(); ipv6 != nil{
			_,err = Conn.Write(append(append([]byte{0x05,0x01,0x00,0x04},[]byte(ipv4)...),p...))
			checkErr(err)
		}else{
			panic(errors.New("未知错误！"))
		}
	}else{
		// 域名 ATYP =  0x03
		copy(raw[0:4],[]byte{0x05,0x01,0x00,0x03})
		length := len([]byte(addr))
		if length > 0xff {
			panic(errors.New("目标地址超过最大长度！"))
		}
		raw[4] = byte(length)
		copy(raw[5:7 + length],append([]byte(addr),p...))
		_,err = Conn.Write(raw[:7 + length])
		checkErr(err)
	}


	n,err = Conn.Read(raw[:])
	if err == io.EOF {
		panic(errors.New("远程服务器断开链接！"))
	}
	checkErr(err)

	if raw[0] != 0x05 || raw[2] != 0x00{
		panic(errors.New("不支持代理服务器！"))
	}

	switch raw[1] {
	case 0x00:
		break
	case 0x01:
		panic(errors.New("服务器错误:X'01' general SOCKS server failure"))
	case 0x02:
		panic(errors.New("服务器错误:X'02' connection not allowed by ruleset"))
	case 0x03:
		panic(errors.New("服务器错误:X'03' Network unreachable"))
	case 0x04:
		panic(errors.New("服务器错误:X'04' Host unreachable"))
	case 0x05:
		panic(errors.New("服务器错误:X'05' Connection refused"))
	case 0x06:
		panic(errors.New("服务器错误:X'06' TTL expired"))
	case 0x07:
		panic(errors.New("服务器错误:X'07' Command not supported"))
	case 0x08:
		panic(errors.New("服务器错误:X'08' Address type not supported"))
	case 0x09:
		panic(errors.New("服务器错误:X'09' to X'FF' unassigned"))
	}
	switch raw[3] {
	case 0x01:
		BindHost := net.IPv4(raw[4], raw[5], raw[6], raw[7]).String()
		BindPort := strconv.Itoa(int(raw[n-2])<<8 | int(raw[n-1]))
		log.Printf("%s <-> %s <-> %s <-> %s",Conn.LocalAddr().String(),Conn.RemoteAddr().String(),net.JoinHostPort(BindHost, BindPort),host)
	case 0x03:
		// TODO： 绑定的是域名
	case 0x04:
		// TODO： 绑定的是IPv6地址
	default:
		// TODO： 未知类型
		panic(errors.New("未知代理类型"))
	}

	return Conn,nil
}
func doDirect(host string) (Conn net.Conn,err error) {
	defer func() {
		if e := recover(); e != nil{
			err = fmt.Errorf("socks5 proxy error:%s",e)
			Conn = nil
		}
	}()

	Conn,err = net.DialTimeout("tcp",address(host),timeout)
	log.Printf("连接到远程主机：%s",address(host))
	return
}
func address(host string) string {
	var address string
	if strings.Index(host,"://") != -1{
		uinfo,err := url.Parse(host)
		checkErr(err)
		address = uinfo.Host
	}else{
		address =host
	}

	if strings.IndexByte(address,':') == -1{
		address = net.JoinHostPort(address,"80")
	}
	return address
}

func readLineSlice(r *bufio.Reader) ([]byte, error) {
	var line []byte
	for {
		l, more, err := r.ReadLine()
		if err != nil {
			return nil, err
		}
		// Avoid the copy if the first call produced a full line.
		if line == nil && !more {
			return l, nil
		}
		line = append(line, l...)
		if !more {
			break
		}
	}
	return line, nil
}