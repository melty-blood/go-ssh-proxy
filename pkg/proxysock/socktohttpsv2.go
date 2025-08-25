// Deprecated: As of next version 1.2.2
package proxysock

import (
	"context"
	"crypto/tls"
	"fmt"
	"honoka/pkg/confopt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"

	"golang.org/x/net/proxy"
)

const (
	socks5Address = "127.0.0.1:20022" // SOCKS5 代理地址
	proxyAddress  = "127.0.0.1:7890"  // HTTP/HTTPS 代理地址
)

// Deprecated: As of next version 1.2.2
func TempAA(conf *confopt.Config) error {
	// 创建 SOCKS5 拨号器
	dialer, err := proxy.SOCKS5("tcp", socks5Address, nil, proxy.Direct)
	if err != nil {
		log.Fatal("Error creating SOCKS5 dialer:", err)
	}

	// 创建 HTTP 传输器，使用 SOCKS5 拨号器
	httpTransport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return dialer.Dial(network, addr)
		},
	}

	httpsTransport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return dialer.Dial(network, addr)
		},
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // 注意：生产环境不应跳过证书验证
	}

	// 创建 HTTP 客户端
	httpClient := &http.Client{Transport: httpTransport}
	httpsClient := &http.Client{Transport: httpsTransport}

	// 创建 HTTP 服务器
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("++++++1111111 ", r.Method)
		if true {
			handleHTTPSV1(w, r, dialer)
			return
		}

		req, err := http.NewRequest(r.Method, r.URL.String(), r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		req.Header = r.Header
		var resp *http.Response
		if r.URL.Scheme == "https" {
			resp, err = httpsClient.Do(req)
		} else {
			resp, err = httpClient.Do(req)
		}

		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()

		// 将响应头复制到客户端
		for k, vv := range resp.Header {
			for _, v := range vv {
				w.Header().Add(k, v)
			}
		}
		w.WriteHeader(resp.StatusCode)
		_, err = io.Copy(w, resp.Body)
		if err != nil {
			log.Println("Error copying response body:", err)
		}

	})

	fmt.Println("Proxy server listening on", proxyAddress)
	log.Fatal(http.ListenAndServe(proxyAddress, nil))
	return nil
}

func handleHTTPSV1(w http.ResponseWriter, r *http.Request, dialer proxy.Dialer) {
	destConn, err := dialer.Dial("tcp", r.Host)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	defer destConn.Close()

	w.WriteHeader(http.StatusOK)
	hj, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "http.Hijacker not supported", http.StatusInternalServerError)
		return
	}
	clientConn, _, err := hj.Hijack()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer clientConn.Close()

	// 开始双向传输数据
	go transfer(clientConn, destConn)
	go transfer(destConn, clientConn)
}

func transfer(destination io.WriteCloser, source io.ReadCloser) {
	defer destination.Close()
	defer source.Close()
	_, err := io.Copy(destination, source)
	if err != nil {
		// 通常是客户端或服务器断开连接
		if ne, ok := err.(net.Error); ok && ne.Timeout() {
			return // ignore timeouts
		}
		if !strings.Contains(err.Error(), "use of closed network connection") {
			log.Println("Transfer error:", err)
		}
	}
}
