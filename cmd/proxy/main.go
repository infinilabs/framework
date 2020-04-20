/*
#!/usr/bin/env gorun
@author :yinzhengjie
Blog:http://www.cnblogs.com/yinzhengjie/tag/GO%E8%AF%AD%E8%A8%80%E7%9A%84%E8%BF%9B%E9%98%B6%E4%B9%8B%E8%B7%AF/
EMAIL:y1053419035@qq.com
*/

package main

import (
	"crypto/md5"
	"crypto/rc4"
	"flag"
	"io"
	"log"
	"net"
	"sync"
)

/*
1>安装switchyomega软件；
2.Golang编写代理服务
3>以下代码是一个TCP代理程序,它是一个通用的四层代理。
4.了解什么是透明代理。
*/

var (
	target = flag.String("bind", "www.qq.com:80", "bind == host:port") //第一个参数是定义关键字，第二个参数是告诉让用户输入相应的主机+端口。第三个参数是告诉用户使用方法
)

func Crypto(w io.Writer, f io.Reader, key string) {
	md5sum := md5.Sum([]byte(key))                  //首先对秘钥进行MD5运算，使得秘钥的长度更长！
	cipher, err := rc4.NewCipher([]byte(md5sum[:])) //定义一个加密器
	if err != nil {
		log.Fatal(err)
	}
	buf := make([]byte, 4096) //定义一个指定大小的容器。
	for {
		n, err := f.Read(buf) //将“f”的内容读取到“buf”中去，但是读取的大小是固定的哟！
		if err == io.EOF {    //当读取到结尾的时候就中止循环，我们这里不考虑其他异常！
			break
		}
		src := buf[:n]                //将读取到的内容取出来，即都是字节。而非一个长度数字。
		cipher.XORKeyStream(src, src) //进行原地加密
		w.Write(src)                  //将加密后的数据写入到“w”中。
	}
}

func handle_conn(conn net.Conn) {
	var (
		remote net.Conn //定义远端的服务器连接。
		err    error
	)
	remote, err = net.Dial("tcp", *target) //建立到目标服务器的连接。
	if err != nil {
		log.Print(err)
		conn.Close()
		return
	}

	wg := new(sync.WaitGroup)
	wg.Add(2)

	go func() {
		defer wg.Done()
		io.Copy(remote, conn) //读取原地址请求（conn），然后将读取到的数据发送给目标主机。
		remote.Close()
	}()

	go func() {
		defer conn.Close()
		io.Copy(conn, remote) //与上面相反，就是讲目标主机的数据返回给客户端。
		conn.Close()
	}()

	wg.Wait()

}

func main() {
	flag.Parse()
	listener, err := net.Listen("tcp", ":8888")
	if err != nil {
		log.Fatal(err)
	}
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Fatal(err)
		}
		go handle_conn(conn)
	}
}

/*
用法一：
    不指定参数，会用代码默认的参数。
    服务端：[root@yinzhengjie yinzhengjie]# go run tcp_proxy.go
    客户端：[root@yinzhengjie ~]# curl -v 127.0.0.1:8888

用法二：
    指定参数时需要知名IP和端口号。
    服务端：[root@yinzhengjie yinzhengjie]# go run tcp_proxy.go --bind=127.0.0.1:22
    客户端：[root@yinzhengjie ~]# ssh -p 8888 127.0.0.1

用法三：
    星球大战代理：
    服务端：[root@yinzhengjie yinzhengjie]# go run tcp_proxy.go --bind=towel.blinkenlights.nl:23
    客户端：[root@yinzhengjie ~]# telnet 127.0.0.1 8888
*/
