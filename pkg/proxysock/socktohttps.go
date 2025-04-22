package proxysock

import (
	"bufio"
	"fmt"
	"honoka/pkg/confopt"
	"io"
	"log"
	"net"
	"strings"
)

func SocksTOHttps(conf *confopt.Config) error {

	// SOCKS5 代理地址（由 SSH -ND 创建）
	socks5Addr := conf.SockTOHttp.SockAddr // 替换为你的 SOCKS5 代理地址

	// 启动 HTTP 代理服务
	httpProxyAddr := conf.SockTOHttp.TOHttp

	// 启动 HTTP/HTTPS 代理
	err := StartHTTPProxy(httpProxyAddr, socks5Addr)
	if err != nil {
		return err
	}
	log.Println("socks to http done")
	return nil
}

// 启动 HTTP 和 HTTPS 代理服务器
func StartHTTPProxy(httpProxyAddr string, socksAddr string) error {
	listener, err := net.Listen("tcp", httpProxyAddr)
	if err != nil {
		return fmt.Errorf("failed to listen on port %s: %v", httpProxyAddr, err)
	}
	defer listener.Close()

	log.Printf("HTTP proxy listening on %s, forwarding via SOCKS5 at %s \n", httpProxyAddr, socksAddr)

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("Failed to accept connection: %v", err)
			continue
		}

		go handleHTTPProxyConnection(conn, socksAddr)
	}
}

// 处理每个客户端的 HTTP/HTTPS 代理请求
func handleHTTPProxyConnection(clientConn net.Conn, socksAddr string) {
	defer clientConn.Close()

	reader := bufio.NewReader(clientConn)
	// 读取客户端请求
	requestLine, err := reader.ReadString('\n')
	if err != nil {
		log.Printf("Failed to read request line: %v", err)
		return
	}

	// 解析 HTTP 请求
	requestParts := strings.Split(requestLine, " ")
	if len(requestParts) < 3 {
		log.Printf("Invalid request line: %s", requestLine)
		return
	}

	method := requestParts[0]
	target := requestParts[1]
	protocol := requestParts[2]
	fmt.Println("+++++++++++++++++ ", requestParts)
	if method == "CONNECT" {
		// HTTPS 隧道请求
		handleHTTPS(clientConn, target, protocol, socksAddr)
	} else {
		// 普通 HTTP 请求
		handleHTTP(clientConn, requestLine, reader, socksAddr)
	}
}

// 处理 HTTPS 隧道请求
func handleHTTPS(clientConn net.Conn, target, protocol, socksAddr string) {
	// 告知客户端隧道已建立
	log.Println("handleHTTPS listen start ", clientConn, target, protocol, socksAddr, "200 Connection Established")

	// 通过 SOCKS5 建立连接到目标服务器
	socksConn, err := net.Dial("tcp", socksAddr)
	if err != nil {
		log.Printf("Failed to connect to SOCKS5 proxy: %v", err)
		return
	}
	defer socksConn.Close()

	// 转发客户端与目标服务器之间的流量
	go io.Copy(socksConn, clientConn)
	io.Copy(clientConn, socksConn)
}

// 处理 HTTP 请求
func handleHTTP(clientConn net.Conn, requestLine string, reader *bufio.Reader, socksAddr string) {
	// 通过 SOCKS5 建立连接到目标服务器
	socksConn, err := net.Dial("tcp", socksAddr)
	if err != nil {
		log.Printf("Failed to connect to SOCKS5 proxy: %v", err)
		return
	}
	defer socksConn.Close()

	// 转发 HTTP 请求到目标服务器
	_, err = socksConn.Write([]byte(requestLine))
	if err != nil {
		log.Printf("Failed to forward request: %v", err)
		return
	}

	// 转发剩余的数据
	go io.Copy(socksConn, reader)
	io.Copy(clientConn, socksConn)
}
