package proxysock

import (
	"context"
	"errors"
	"honoka/pkg/confopt"
	"net"
	"net/http"
	"time"

	"github.com/elazarl/goproxy"
	"golang.org/x/net/proxy"
)

func SocksToHttps(conf *confopt.Config) error {
	logp := NewPrintLog("SocksToHttps", "")

	socksAddr := conf.SockToHttp.SockAddr
	// 创建 SOCKS5 dialer
	dialer, err := proxy.SOCKS5("tcp", socksAddr, nil, proxy.Direct)
	if err != nil {
		return errors.New("SocksToHttps-failed to create socks5 dialer:" + err.Error())
	}

	// 实现 DialContext 签名的函数
	transport := &http.Transport{
		// 把所有出站连接通过 SOCKS5 dialer 建立
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return dialer.Dial(network, addr)
		},

		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	// goproxy 进行 HTTP 代理（并处理 CONNECT 用于 HTTPS 隧道）
	proxyServer := goproxy.NewProxyHttpServer()
	proxyServer.Tr = transport
	proxyServer.Verbose = false

	logp.PrintF("Starting HTTP(S) socks5 on %s -> proxy on %s", socksAddr, conf.SockToHttp.ToHttp)
	if err := http.ListenAndServe(conf.SockToHttp.ToHttp, proxyServer); err != nil {
		return errors.New("SocksToHttps-proxy server failed:" + err.Error())
	}
	return nil
}
