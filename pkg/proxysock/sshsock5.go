// socks_ssh_forward.go
package proxysock

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"honoka/pkg/confopt"
	"io"
	"log"
	"net"
	"os"
	"time"

	"golang.org/x/crypto/ssh"
)

// readExact reads exactly n bytes or returns error
func readExact(conn net.Conn, n int) ([]byte, error) {
	buf := make([]byte, n)
	_, err := io.ReadFull(conn, buf)
	return buf, err
}

func handleSocksConn(local net.Conn, sshClient *ssh.Client) {
	defer local.Close()

	// SOCKS5 handshake
	hdr, err := readExact(local, 2)
	if err != nil {
		log.Printf("handleSocksConn-handshake read error: %v", err)
		return
	}
	if hdr[0] != 0x05 {
		log.Printf("handleSocksConn-unsupported socks version: %v", hdr[0])
		return
	}
	nmethods := int(hdr[1])
	if _, err = readExact(local, nmethods); err != nil {
		log.Printf("handleSocksConn-read methods error: %v", err)
		return
	}
	// reply: no auth (0x00)
	if _, err = local.Write([]byte{0x05, 0x00}); err != nil {
		log.Printf("handleSocksConn-write handshake reply error: %v", err)
		return
	}

	// read request
	reqHead, err := readExact(local, 4)
	if err != nil {
		log.Printf("handleSocksConn-read request header error: %v", err)
		return
	}
	if reqHead[0] != 0x05 {
		log.Printf("handleSocksConn-unsupported request version: %v", reqHead[0])
		return
	}
	cmd := reqHead[1]
	// rsv := reqHead[2]
	atyp := reqHead[3]

	if cmd != 0x01 {
		// only CONNECT supported
		// reply: command not supported (0x07)
		local.Write([]byte{0x05, 0x07, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
		log.Printf("handleSocksConn-unsupported socks command: %v", cmd)
		return
	}

	var host string
	switch atyp {
	case 0x01:
		// IPv4
		b, err := readExact(local, 4)
		if err != nil {
			log.Printf("handleSocksConn-read ipv4 error: %v", err)
			return
		}
		host = net.IP(b).String()
	case 0x03:
		// domain
		lenb, err := readExact(local, 1)
		if err != nil {
			log.Printf("handleSocksConn-read domain length error: %v", err)
			return
		}
		dlen := int(lenb[0])
		db, err := readExact(local, dlen)
		if err != nil {
			log.Printf("handleSocksConn-read domain error: %v", err)
			return
		}
		host = string(db)
	case 0x04:
		// IPv6
		b, err := readExact(local, 16)
		if err != nil {
			log.Printf("handleSocksConn-read ipv6 error: %v", err)
			return
		}
		host = net.IP(b).String()
	default:
		local.Write([]byte{0x05, 0x08, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
		log.Printf("handleSocksConn-unknown atyp: %v", atyp)
		return
	}
	// port
	pb, err := readExact(local, 2)
	if err != nil {
		log.Printf("handleSocksConn-read port error: %v", err)
		return
	}
	port := binary.BigEndian.Uint16(pb)
	dest := fmt.Sprintf("%s:%d", host, port)
	log.Printf("handleSocksConn-SOCKS CONNECT: %s", dest)

	// Use sshClient.Dial to create connection from remote side to dest
	remote, err := sshClient.Dial("tcp", dest)
	if err != nil {
		// reply: general failure
		local.Write([]byte{0x05, 0x01, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
		log.Printf("handleSocksConn-ssh dial to %s failed: %v", dest, err)
		return
	}
	// reply: success (bind addr 0.0.0.0:0)
	_, _ = local.Write([]byte{0x05, 0x00, 0x00, 0x01, 0, 0, 0, 0, 0, 0})

	// pipe data both ways
	done := make(chan struct{}, 2)
	go func() {
		_, err = io.Copy(remote, local)
		remote.Close()
		done <- struct{}{}
		if err != nil {
			log.Println("handleSocksConn-Copy remote->local err:", err)
		}
	}()
	go func() {
		_, err = io.Copy(local, remote)
		local.Close()
		if err != nil {
			log.Println("handleSocksConn-Copy local->remote err:", err)
		}
		done <- struct{}{}
	}()
	<-done
	<-done
	log.Printf("handleSocksConn-connection %s closed", dest)
}

// TIP: ssh -ND 22122
func RunSSHSock5(ctx context.Context, conf *confopt.Config, onlineChan chan string) error {
	proxyOpt := conf.SockProxy
	var (
		user    = proxyOpt.ServerUser
		server  = proxyOpt.ServerHost
		keyFile = proxyOpt.ServerPriKey
		// keyPass   = flag.String("keypass", "", "private key passphrase (optional)")
		password  = proxyOpt.ServerPassword
		listen    = proxyOpt.Local
		insecure  = false
		keepAlive = 6
	)

	hostKeyCallback := ssh.InsecureIgnoreHostKey()
	if !insecure {
		hostKeyCallback = ssh.InsecureIgnoreHostKey()
	}
	cfg := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.Password(password),
		},
		HostKeyCallback: hostKeyCallback,
		Timeout:         20 * time.Second,
	}
	if len(keyFile) > 0 {
		priKey, err := PublicKeyAuth(keyFile)
		if err != nil {
			return errors.New("RunSSHSock5: load key failed:" + err.Error())
		}
		cfg.Auth = []ssh.AuthMethod{
			priKey,
		}
	}

	log.Printf("RunSSHSock5: dialing ssh %s", server)
	sshClient, err := ssh.Dial("tcp", server, cfg)
	defer func() {
		if err != nil {
			onlineChan <- "RestartSSHSockProxy"
		}
	}()
	if err != nil {
		return errors.New("RunSSHSock5: ssh dial failed:" + err.Error())
	}
	defer sshClient.Close()
	log.Printf("RunSSHSock5: ssh connection established to %s", server)

	// optional keepalive
	if keepAlive > 0 {
		go keepAliveSendReq(sshClient, keepAlive)
	}

	ln, err := net.Listen("tcp", listen)
	if err != nil {
		return errors.New("RunSSHSock5: listen: " + listen + " | err: " + err.Error())
	}
	log.Printf("RunSSHSock5: SOCKS5 listening on %s (forward via %s) \n", listen, server)
	go func() {
		time.Sleep(time.Second)
		onlineChan <- "RunProxyServer"
	}()
	go sock5Cancel(ctx, ln, sshClient)
	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Printf("RunSSHSock5: accept error: %v", err)
			break
		}
		go handleSocksConn(conn, sshClient)
	}
	return err
}

func sock5Cancel(ctx context.Context, listen net.Listener, sshClient *ssh.Client) {
	for {
		select {
		case <-ctx.Done():
			listen.Close()
			sshClient.Close()
			log.Println("sock5Cancel: ctx.Done")
			return
		default:
			time.Sleep(time.Second)
		}

	}
}

func keepAliveSendReq(sshClient *ssh.Client, keepAlive int) {
	var (
		sendOk      bool
		sendResByte []byte
		err         error
	)
	req := struct {
		Addr    string
		Port    uint32
		Payload string
	}{
		"127.0.0.1",
		uint32(0),
		"kotori",
	}
	var reply struct {
		SomeField uint32
	}
	timeSecond := time.Duration(keepAlive) * time.Second
	time.Sleep(time.Second)
	for {
		sendOk, sendResByte, err = sshClient.SendRequest("tcpip-forward", true, ssh.Marshal(&req))
		if err != nil {
			log.Println("keepAliveSendReq->Error:RunSSHSock5: sshClient.SendRequest:", sendOk, " | ", sendResByte, reply, " | ", err)
			log.Printf("keepAliveSendReq->Error:keepalive failed: %v", err)
			return
		}
		// err = ssh.Unmarshal(sendResByte, &reply)
		// log.Println("keepAliveSendReq->err: ", err, reply)
		time.Sleep(timeSecond)
	}
}

func PublicKeyAuthFuncTemp(keyPath string, passphrase string) (ssh.AuthMethod, error) {
	keyBytes, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, err
	}
	var signer ssh.Signer
	if passphrase == "" {
		signer, err = ssh.ParsePrivateKey(keyBytes)
	} else {
		signer, err = ssh.ParsePrivateKeyWithPassphrase(keyBytes, []byte(passphrase))
	}
	if err != nil {
		return nil, err
	}
	return ssh.PublicKeys(signer), nil
}
