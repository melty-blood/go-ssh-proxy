package proxysock

import (
	"context"
	"errors"
	"honoka/pkg/confopt"
	"io"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

type optSSHToServer struct {
	listenLocal     chan string
	baseConnAction  chan string
	listenHasAction chan string
	localToRemote   chan int
	logp            *PrintLog
}

// ssh -NL 20022:6.106.4.5:22 -J TypeMoon ADMINISTRATOR@10.42.211.143 -p 8606

func PublicKeyAuth(priFile string, passphrase string) (ssh.AuthMethod, error) {
	priKey, err := os.ReadFile(priFile)
	if err != nil {
		log.Fatalln("read prikey fail:", err)
		return nil, err
	}

	signer, err := ssh.ParsePrivateKey(priKey)
	if passphrase != "" {
		signer, err = ssh.ParsePrivateKeyWithPassphrase(priKey, []byte(passphrase))
	}
	if err != nil {
		log.Fatalln("parse prikey fail:", err)
		return nil, err
	}
	return ssh.PublicKeys(signer), nil
}

func sshToServerByJump(baseCtx context.Context, serverName string, sshconf *confopt.SSHConfig) error {
	var (
		err  error
		logp = NewPrintLog("sshToServer", "")
	)
	// 目标服务器配置
	targetHost := sshconf.ServerHost
	// 配置目标服务器的 SSH 客户端
	targetConfig := &ssh.ClientConfig{
		User: sshconf.ServerUser,
		Auth: []ssh.AuthMethod{
			ssh.Password(sshconf.ServerPassword),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}
	if len(sshconf.ServerPriKey) > 0 {
		priKey, _ := PublicKeyAuth(sshconf.ServerPriKey, sshconf.ServerPriPass)
		targetConfig.Auth = []ssh.AuthMethod{
			priKey,
		}
	}

	// 本地端口转发设置
	localHostPort := sshconf.Local
	remoteAddr := sshconf.Proxy

	// 跳板机配置
	jumpHost := sshconf.JumpHost
	// 配置跳板机的 SSH 客户端
	// HostKeyCallback 安全的回调
	jumpConfig := &ssh.ClientConfig{
		User: sshconf.JumpUser,
		Auth: []ssh.AuthMethod{
			ssh.Password(sshconf.JumpPassword),
		},
		Timeout:         6 * time.Second,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}
	if len(sshconf.JumpPriKey) > 0 {
		priKey, _ := PublicKeyAuth(sshconf.JumpPriKey, sshconf.JumpPriPass)
		jumpConfig.Auth = []ssh.AuthMethod{
			priKey,
		}
	}

	// 连接到跳板机
	jumpConn, err := ssh.Dial("tcp", jumpHost, jumpConfig)
	if err != nil {
		logp.Print("Error Failed to connect to jump host:", err)
		return err
	}
	defer jumpConn.Close()

	// 通过跳板机连接目标服务器
	targetConn, err := jumpConn.Dial("tcp", targetHost)
	if err != nil {
		logp.Print("Error Failed to connect target host through jump host:", err, serverName)
		return err
	}

	// 创建目标服务器的 SSH 客户端
	targetSSHConn, chans, reqs, err := ssh.NewClientConn(targetConn, targetHost, targetConfig)
	if err != nil {
		logp.Print("Error Failed to establish SSH connection to target host:", err)
		return err
	}
	defer targetSSHConn.Close()

	targetClient := ssh.NewClient(targetSSHConn, chans, reqs)
	defer targetClient.Close()

	// 设置本地监听端口
	localListener, err := net.Listen("tcp", localHostPort)
	if err != nil {
		logp.Print("Error Failed to set up local listener:", err)
		return err
	}
	defer localListener.Close()

	logp.PrintF("->Forwarding %s to %s >> %s, jump start", localHostPort, remoteAddr, serverName)
	listenChan := make(chan string, 66)
	defer close(listenChan)

	// 若一定时间没有客户端接入主动退出重新连接
	baseConnActionChan := make(chan string, 266)
	defer close(baseConnActionChan)
	go baseConnCancel(baseCtx, baseConnActionChan, serverName, localListener)

	// 客户端每次发送则进行记录并告诉任务重新计时
	listenHasActionChan := make(chan string, 266)
	defer close(listenHasActionChan)
	localToRemoteChan := make(chan int, 266)
	defer close(localToRemoteChan)

	// 用于监听客户端连接后是否在一定时间内有响应, 如果没有则需要断开
	go connTimeoutCancel(baseCtx, serverName, listenHasActionChan, localListener)

	// 处理本地连接并转发
	opt := &optSSHToServer{
		listenLocal:     listenChan,
		baseConnAction:  baseConnActionChan,
		listenHasAction: listenHasActionChan,
		localToRemote:   localToRemoteChan,
	}
	err = startSSHCon(baseCtx, localListener, targetClient, sshconf, opt)
	if err != nil {
		return err
	}
	return nil
	// 处理本地连接并转发
	// for {
	// 	localConn, err := localListener.Accept()
	// 	log.Println("start localListener.Accept(): ", localConn, err, serverName)
	// 	if err != nil && strings.Contains(err.Error(), "use of closed network connection") {
	// 		return nil
	// 	}
	// 	defer localConn.Close()

	// 	select {
	// 	case listenChanStr, ok := <-listenChan:
	// 		log.Println("listenChan is close sshProxy restart:", listenChanStr, ok)
	// 		if !ok {
	// 			return nil
	// 		}
	// 		if listenChanStr != "yes" {
	// 			return errors.New(listenChanStr)
	// 		}
	// 		return nil
	// 	default:
	// 		log.Println("listenChan nothing data")
	// 	}

	// 	if err != nil {
	// 		// 不退出, 关闭本次链接
	// 		log.Println("Error Failed to accept local connection: ", err)
	// 		clientNum++
	// 		if clientNum >= 10 {
	// 			return err
	// 		}
	// 		continue
	// 	}

	// 	go func() {
	// 		log.Println("go func start targetClient.Dial: ", remoteAddr)
	// 		// 建立到目标服务器的连接
	// 		remoteConn, err := targetClient.Dial("tcp", remoteAddr)
	// 		log.Println("start targetClient.Dial: ", err)
	// 		if err != nil {
	// 			log.Printf("Error %s Failed to connect to remote address %s: %v", serverName, remoteAddr, err)
	// 			listenChan <- "go func targetClient.Dial error: " + err.Error()
	// 			return
	// 		}
	// 		defer remoteConn.Close()

	// 		readCtx, readCancel := context.WithCancel(context.Background())
	// 		defer readCancel()

	// 		// 当前goroutine不能放在下面的for中, 因为Read会阻塞导致无法执行到
	// 		go connClose(readCtx, serverName, localConn, remoteConn)

	// 		// +++++++++++++++++ client => server START +++++++++++++++++
	// 		go func() {
	// 			defer func() {
	// 				remoteConn.Close()
	// 				localConn.Close()
	// 				readCancel()
	// 			}()

	// 			// 判断通道是否有关闭的
	// 			if !channelIsCloseAny(baseConnActionChan, listenHasActionChan, listenChan) || !channelIsCloseAny(localToRemoteChan) {
	// 				return
	// 			}
	// 			for {
	// 				buf := make([]byte, 65536)
	// 				n, err := localConn.Read(buf)
	// 				if err != nil && err != io.EOF {
	// 					log.Printf("Error reading from client: %v, %s", err, serverName)
	// 					return
	// 				}
	// 				if n > 0 {
	// 					log.Printf("Sent to server: %d bytes, %s", n, serverName)
	// 					localToRemoteChan <- n
	// 					_, err := remoteConn.Write(buf[:n])
	// 					if err == io.EOF {
	// 						log.Printf("Error remoteConn.Write to remote server EOF: %v, %s", err, serverName)
	// 						localListener.Close()
	// 						listenChan <- "yes"
	// 						return
	// 					}
	// 					if err != nil && err != io.EOF {
	// 						log.Printf("Error writing to remote server: %v", err)
	// 						return
	// 					}
	// 					if len(listenHasActionChan) > 1 {
	// 						log.Println("listenHasActionChan_baseConnChan count continue stop add: ", len(listenHasActionChan), len(baseConnActionChan))
	// 						continue
	// 					}
	// 					actionChanStr = serverName + "_" + strconv.Itoa(n)
	// 					baseConnActionChan <- actionChanStr
	// 					listenHasActionChan <- actionChanStr
	// 				}
	// 				if err == io.EOF {
	// 					log.Println("client=>server go func: err is io.EOF => return by " + serverName)
	// 					return
	// 				}

	// 			}
	// 		}()
	// 		go listenLocalToRemote(readCtx, localToRemoteChan, sshconf)

	// 		_, err = io.Copy(localConn, remoteConn)
	// 		log.Println("sshToServerByJump io.Copy:", err, serverName)
	// 		// +++++++++++++++++ server => client END +++++++++++++++++

	// 	}()

	// 	// 防止一直不跳出循环导致客户端无法重新连接代理
	// 	for range 2 {
	// 		// for {
	// 		select {
	// 		case errStr = <-listenChan:
	// 			log.Println(serverName + " listen: " + errStr)
	// 			if errStr != "yes" {
	// 				return errors.New(errStr)
	// 			}
	// 			return nil
	// 		default:
	// 			log.Println(serverName + " listen: no error")
	// 			time.Sleep(1 * time.Second)
	// 		}
	// 	}
	// }
}

func baseConnCancel(baseCtx context.Context, connActionChan chan string, serverName string, localConn net.Listener) {
	logp := NewPrintLog("baseConnCancel", serverName)

	time.Sleep(6 * time.Second)
	var baseNumInt int = 0
	logp.Print("localListener ready close 600s:")
	for {
		select {
		case connActionStr, ok := <-connActionChan:
			if !ok {
				localConn.Close()
				logp.Print("localListener close by connActionChan close!")
				return
			}
			baseNumInt = 0
			logp.Print("connActionChan has new value:", connActionStr)

		case nilStruct, ok := <-baseCtx.Done():
			localConn.Close()
			logp.Print("localListener close by baseCtx Cancel!", nilStruct, ok)
			return

		default:
			time.Sleep(time.Second)
			baseNumInt++
			if baseNumInt > 600 {
				localConn.Close()
				logp.Print("localListener close now!", baseNumInt)

				return
			}
		}
	}
}

func connTimeoutCancel(
	ctx context.Context,
	serverName string,
	hasActionChan chan string,
	localConn net.Listener,
) {
	logp := NewPrintLog("connTimeoutCancel", serverName)
	exitChan := make(chan string, 6)
	defer close(exitChan)

	go func() {
		// 当前goroutine负责监听通道 hasActionChan
		var outInt int = 0
		for {
			select {
			case _, ok := <-ctx.Done():
				exitChan <- "exit_channel connTimeoutCancel ctx close"
				logp.Print("go func select ctx.Done close:", ok)
				return
			case actionStr, ok := <-hasActionChan:
				if !ok {
					logp.Print("go func select hasActionChan is close")
					exitChan <- "exit_channel hasActionChan close"
					return
				}
				logp.PrintF("go func select received <-hasActionChan value: %s \n ", actionStr)
				// 每次发送数据时间重新初始化
				outInt = 0
			default:
				time.Sleep(time.Second)
				outInt++
				if outInt > 360 {
					exitChan <- "exit_with hasActionChan nothing action with seconds: " + strconv.Itoa(outInt)
					return
				}
			}
		}
	}()

	for {
		select {
		case exitStr := <-exitChan:
			localConn.Close()
			logp.Print("cancel success ++++++++", exitStr, "|", ctx.Err())
			return
		default:
			time.Sleep(time.Second)
		}
	}
}

func connClose(ctx context.Context, serverName string, local, remote net.Conn) {
	for {
		select {
		case _, ok := <-ctx.Done():
			remote.Close()
			local.Close()
			log.Println("connClose remoteConn localConn close now,", serverName, ok)
			return
		default:
			time.Sleep(6 * time.Second)
		}
	}
}

func sshToServer(baseCtx context.Context, serverName string, sshconf *confopt.SSHConfig) error {
	var (
		err  error
		logp = NewPrintLog("sshToServer", "")
	)
	// 目标服务器配置
	targetHost := sshconf.ServerHost
	// 配置目标服务器的 SSH 客户端
	targetConfig := &ssh.ClientConfig{
		User: sshconf.ServerUser,
		Auth: []ssh.AuthMethod{
			ssh.Password(sshconf.ServerPassword),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // 根据需求设置更安全的回调
	}
	if len(sshconf.ServerPriKey) > 0 {
		priKey, _ := PublicKeyAuth(sshconf.ServerPriKey, sshconf.ServerPriPass)
		targetConfig.Auth = []ssh.AuthMethod{
			priKey,
		}
	}
	// 本地端口转发设置
	localHostPort := sshconf.Local
	remoteAddr := sshconf.Proxy

	// 连接到目标服务器
	targetConn, err := ssh.Dial("tcp", targetHost, targetConfig)
	if err != nil {
		logp.PrintF("Error Failed to connect to target host: %v", err)
		return err
	}
	defer targetConn.Close()

	// 设置本地监听端口
	localListener, err := net.Listen("tcp", localHostPort)
	if err != nil {
		logp.PrintF("Error Failed to set up local listener: %v", err)
		return err
	}
	defer localListener.Close()

	logp.PrintF("->Forwarding %s to %s >> %s,no jump", localHostPort, remoteAddr, serverName)
	listenChan := make(chan string, 66)
	defer close(listenChan)

	// 若一定时间没有客户端接入应该主动退出重新连接
	baseConnActionChan := make(chan string, 266)
	defer close(baseConnActionChan)
	go baseConnCancel(baseCtx, baseConnActionChan, serverName, localListener)

	// 客户端每次发送则进行记录并告诉任务重新计时
	listenHasActionChan := make(chan string, 266)
	defer close(listenHasActionChan)
	localToRemoteChan := make(chan int, 266)
	defer close(localToRemoteChan)

	// 用于监听客户端连接后是否在一定时间内有响应, 如果没有则需要断开
	go connTimeoutCancel(baseCtx, serverName, listenHasActionChan, localListener)

	// 处理本地连接并转发
	opt := &optSSHToServer{
		listenLocal:     listenChan,
		baseConnAction:  baseConnActionChan,
		listenHasAction: listenHasActionChan,
		localToRemote:   localToRemoteChan,
		logp:            logp,
	}
	err = startSSHCon(baseCtx, localListener, targetConn, sshconf, opt)
	if err != nil {
		return err
	}
	return nil
}

func channelIsCloseAny[CN ~chan VAL, VAL string | int](chanArr ...CN) bool {
	for _, chanVal := range chanArr {
		select {
		case _, ok := <-chanVal:
			if !ok {
				return false
			}
		default:
		}
	}
	return true
}

func startSSHCon(
	ctx context.Context,
	localListener net.Listener,
	targetConn *ssh.Client,
	sshConf *confopt.SSHConfig,
	opt *optSSHToServer,
) error {
	var (
		// err        error
		clientNum             int
		errStr, actionChanStr string
		serverName            string = sshConf.ServerName
		remoteAddr            string = sshConf.Proxy

		listenChan          = opt.listenLocal
		baseConnActionChan  = opt.baseConnAction
		listenHasActionChan = opt.listenHasAction
		localToRemoteChan   = opt.localToRemote

		logp = opt.logp
	)
	if logp == nil {
		isJump := ""
		if sshConf.NeedJump {
			isJump = "jump"
		}
		logp = NewPrintLog("opt logp lost"+serverName, isJump)
	}

	for {
		localConn, err := localListener.Accept()
		logp.Print("localListener.Accept():", localConn, err, serverName)
		if err != nil && strings.Contains(err.Error(), "use of closed network connection") {
			return nil
		}
		defer localConn.Close()

		select {
		case listenChanStr, ok := <-listenChan:
			logp.Print("listenChan is close, ready restart:", listenChanStr, ok, serverName)
			if !ok {
				return nil
			}
			if listenChanStr != "yes" {
				return errors.New(listenChanStr)
			}
			return nil
		default:
			logp.Print("listenChan nothing data")
		}

		if err != nil {
			// 不退出, 关闭本次链接
			logp.Print("Error Failed to accept local connection:", err)
			clientNum++
			if clientNum >= 10 {
				return err
			}
			continue
		}

		go func() {
			logp.Print("go func start targetClient.Dial:", remoteAddr, serverName)
			// 建立到目标服务器的连接
			remoteConn, err := targetConn.Dial("tcp", remoteAddr)
			logp.Print("go func after targetConn.Dial:", err)
			if err != nil {
				logp.Print("Error Failed targetConn connect to remote addr:", serverName, remoteAddr, err)
				listenChan <- "go func targetConn.Dial error: " + err.Error()
				return
			}
			defer remoteConn.Close()

			readCtx, readCancel := context.WithCancel(ctx)
			defer readCancel()

			// 当前goroutine不能放在下面的for中, 因为Read会阻塞导致无法执行到
			go connClose(readCtx, serverName, localConn, remoteConn)

			go func() {
				defer func() {
					remoteConn.Close()
					localConn.Close()
					readCancel()
				}()

				for {
					// 判断通道是否有关闭的
					if !channelIsCloseAny(baseConnActionChan, listenHasActionChan, listenChan) || !channelIsCloseAny(localToRemoteChan) {
						return
					}

					buf := make([]byte, 65536)
					n, err := localConn.Read(buf)
					if err != nil && err != io.EOF {
						logp.PrintF("Error reading from client: %v, %s", err, serverName)
						return
					}
					if n > 0 {
						logp.PrintF("local Sent to server: %d bytes, %s", n, serverName)
						localToRemoteChan <- n
						_, err := remoteConn.Write(buf[:n])
						if err == io.EOF {
							logp.PrintF("Error remoteConn.Write to remote server EOF: %v, %s", err, serverName)
							localListener.Close()
							listenChan <- "yes"
							return
						}
						if err != nil && err != io.EOF {
							logp.PrintF("Error writing to remote server: %v", err)
							return
						}
						if len(listenHasActionChan) > 2 {
							logp.PrintF("listenHasActionChan_baseConnActionChan continue stop add:", len(listenHasActionChan), len(baseConnActionChan))
							continue
						}
						actionChanStr = serverName + "_" + strconv.Itoa(n)
						baseConnActionChan <- actionChanStr
						listenHasActionChan <- actionChanStr
					}
					if err == io.EOF {
						logp.Print("client=>server go func: err is io.EOF => return by:", serverName)
						return
					}

				}
			}()
			go listenLocalToRemote(readCtx, localToRemoteChan, sshConf)

			// 获取发送和接收记录数据流量
			// go func() {
			// 	// 读取客户端 -> 远程服务器的数据
			// 	// clientReader := io.TeeReader(localConn, os.NewFile(uintptr(syscall.Stdin), "/dev/shm/temp_proxy_out"))
			// 	clientReader := io.TeeReader(localConn, os.Stdout) // 将数据拷贝到 stdout
			// 	fmt.Println("+++++++++++ go func listen len: ")
			// 	io.Copy(remoteConn, clientReader)
			// }()
			// +++++++++++++++++ client => server END +++++++++++++++++

			// +++++++++++++++++ server => client START +++++++++++++++++
			// 读取远程服务器 -> 客户端的数据
			// serverReader := io.TeeReader(remoteConn, os.Stdout) // 将数据拷贝到 stdout
			// tempBytes := []byte{}
			// tempBytesInt, _ := serverReader.Read(tempBytes)
			// fmt.Println("+++++++++++ listen len: ", tempBytesInt, " ++++++ ", tempBytes)
			// io.Copy(localConn, serverReader)

			// 转发数据
			// go io.Copy(remoteConn, localConn)
			_, err = io.Copy(localConn, remoteConn)
			logp.Print("sshToServer io.Copy:", err, serverName)
			// +++++++++++++++++ server => client END +++++++++++++++++
		}()

		for range 2 {
			select {
			case errStr = <-listenChan:
				logp.Print("for <-listenChan has err", serverName, errStr)
				if errStr != "yes" {
					return errors.New(errStr)
				}
				return nil
			default:
				logp.Print("listen: no error,", serverName)
				time.Sleep(1 * time.Second)
			}
		}

	}
}

func listenLocalToRemote(ctx context.Context, localRemoteChan chan int, sshconf *confopt.SSHConfig) {
	logp := NewPrintLog("listenLocalToRemote", "")
	serverName := sshconf.ServerName
	for {
		select {
		case _, ok := <-ctx.Done():
			logp.Print("for select: readCtx close by:", serverName, ok)
			return
		case _, ok := <-localRemoteChan:
			if !ok {
				logp.Print("client=>server select: ok is false => localToRemoteChan close by:", serverName)
				return
			}
			// 处理客户端数据到服务器的传输
		default:
			logp.Print("client=>server select default: no error,", serverName)
			time.Sleep(6 * time.Second)
		}
	}
}
