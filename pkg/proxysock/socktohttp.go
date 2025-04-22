package proxysock

import (
	"context"
	"crypto/tls"
	"errors"
	"honoka/pkg/confopt"
	"io"
	"log"
	"net"
	"net/http"

	"golang.org/x/net/proxy"
)

func SocksTOHttp(conf *confopt.Config) error {

	// SOCKS5 代理地址（由 SSH -ND 创建）
	socks5Addr := conf.SockTOHttp.SockAddr // 替换为你的 SOCKS5 代理地址

	// 启动 HTTP 代理服务
	httpProxyAddr := conf.SockTOHttp.TOHttp

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {

		// 通过 SOCKS5 代理转发 HTTP 请求
		err := handleHTTPThroughSOCKS5(socks5Addr, w, r)
		if err != nil {
			http.Error(w, "Failed to proxy request", http.StatusInternalServerError)
			log.Printf("Proxy error: %v", err)
		}
	})

	log.Printf("HTTP proxy listening on %s, forwarding to SOCKS5 %s \n", httpProxyAddr, socks5Addr)
	err := http.ListenAndServe(httpProxyAddr, nil)
	if err != nil {
		log.Println("http.ListenAndServe: ", httpProxyAddr, " | ", err)
		return errors.New(httpProxyAddr + " -> " + err.Error())
	}
	return nil
}

func handleHTTPThroughSOCKS5(socks5Addr string, w http.ResponseWriter, r *http.Request) error {
	// 创建 SOCKS5 代理拨号器
	dialer, err := proxy.SOCKS5("tcp", socks5Addr, nil, proxy.Direct)
	if err != nil {
		return err
	}

	// 创建 HTTP 传输并设置自定义拨号器
	transport := &http.Transport{
		// Dial: dialer.Dial,
		// DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
		// 	return dialer.Dial(network, addr)
		// },
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return dialer.Dial(network, addr)
		},
		// 设置支持 HTTPS 的 TLS 配置
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true, // 如果是自签名证书，可以设置为 true；生产环境请设置为 false
		},
	}

	// 使用新的 Transport 发起请求
	client := &http.Client{
		Transport: transport,
	}
	log.Println("r.URL.String() ", r.URL.String())
	// 转发请求
	req, err := http.NewRequest(r.Method, r.URL.String(), r.Body)
	if err != nil {
		return err
	}
	req.Header = r.Header

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// 转发响应
	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	w.WriteHeader(resp.StatusCode)
	_, err = io.Copy(w, resp.Body)
	return err
}
