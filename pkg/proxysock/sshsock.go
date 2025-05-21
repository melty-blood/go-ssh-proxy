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

// ssh -NL 20022:6.106.4.5:22 -J TypeMoon ADMINISTRATOR@10.42.211.143 -p 8606

func PublicKeyAuth(priFile string) (ssh.AuthMethod, error) {
	priKey, err := os.ReadFile(priFile)
	if err != nil {
		log.Fatalln("read prikey fail: ", err)
		return nil, err
	}

	signer, err := ssh.ParsePrivateKey(priKey)
	if err != nil {
		log.Fatalln("parse prikey fail: ", err)
		return nil, err
	}
	return ssh.PublicKeys(signer), nil
}

func sshToServerByJump(serverName string, sshconf *confopt.SSHConfig) error {
	var (
		clientNum     int
		err           error
		errStr        string
		actionChanStr string = ""
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
		priKey, _ := PublicKeyAuth(sshconf.ServerPriKey)
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
		priKey, _ := PublicKeyAuth(sshconf.JumpPriKey)
		jumpConfig.Auth = []ssh.AuthMethod{
			priKey,
		}
	}

	// 连接到跳板机
	jumpConn, err := ssh.Dial("tcp", jumpHost, jumpConfig)
	if err != nil {
		log.Println("Error Failed to connect to jump host: ", err)
		return err
	}
	defer jumpConn.Close()

	// 通过跳板机连接目标服务器
	targetConn, err := jumpConn.Dial("tcp", targetHost)
	if err != nil {
		log.Println("Error Failed to connect to target host through jump host: ", err, serverName)
		return err
	}

	// 创建目标服务器的 SSH 客户端
	targetSSHConn, chans, reqs, err := ssh.NewClientConn(targetConn, targetHost, targetConfig)
	if err != nil {
		log.Println("Error Failed to establish SSH connection to target host: ", err)
		return err
	}
	defer targetSSHConn.Close()

	targetClient := ssh.NewClient(targetSSHConn, chans, reqs)
	defer targetClient.Close()

	// 设置本地监听端口
	localListener, err := net.Listen("tcp", localHostPort)
	if err != nil {
		log.Println("Error Failed to set up local listener: ", err)
		return err
	}
	defer localListener.Close()

	log.Printf("Forwarding %s to %s via %s", localHostPort, remoteAddr, targetHost)
	listenChan := make(chan string, 66)
	defer close(listenChan)

	// 若一定时间没有客户端接入主动退出重新连接
	baseConnActionChan := make(chan string, 266)
	defer close(baseConnActionChan)
	go baseConnCancel(baseConnActionChan, serverName, localListener)

	// 处理本地连接并转发
	// clientChan := make(chan string, 66)
	// defer close(clientChan)
	for {
		localConn, err := localListener.Accept()
		log.Println("start localListener.Accept(): ", localConn, err)
		if err != nil && strings.Contains(err.Error(), "use of closed network connection") {
			return nil
		}

		select {
		case listenChanStr, ok := <-listenChan:
			if !ok {
				log.Println("listenChan is close sshProxy restart: ")
				return nil
			}
			if listenChanStr != "yes" {
				return errors.New(listenChanStr)
			}
			return nil
		default:
			log.Println("listenChan nothing data")
		}

		if err != nil {
			// 不退出, 关闭本次链接
			log.Println("Error Failed to accept local connection: ", err)
			clientNum++
			if clientNum >= 10 {
				return err
			}
			continue
		}

		// 客户端每次发送则进行记录并告诉任务重新计时
		listenHasActionChan := make(chan string, 166)
		defer close(listenHasActionChan)

		// 用于监听客户端连接后是否在一定时间内有响应, 如果没有则需要断开
		waitCtx, waitCancel := context.WithCancel(context.Background())
		defer waitCancel()
		go connTimeoutCancel(waitCtx, waitCancel, serverName, listenHasActionChan)

		go func() {
			defer localConn.Close()
			log.Println("go func start targetClient.Dial: ", remoteAddr)

			// 建立到目标服务器的连接
			remoteConn, err := targetClient.Dial("tcp", remoteAddr)
			log.Println("start targetClient.Dial: ", err)
			if err != nil {
				log.Printf("Error %s Failed to connect to remote address %s: %v", serverName, remoteAddr, err)
				listenChan <- "go func targetClient.Dial error: " + err.Error()
				return
			}
			defer remoteConn.Close()

			// +++++++++++++++++ client => server START +++++++++++++++++
			localToRemoteChan := make(chan int, 100)
			go func() {
				defer func() {
					close(localToRemoteChan)
					remoteConn.Close()
					localConn.Close()
				}()

				// 当前goroutine不能放在下面的for中, 因为Read会阻塞导致无法执行到
				go connClose(waitCtx, serverName, localConn, remoteConn)

				for {
					buf := make([]byte, 8192)
					n, err := localConn.Read(buf)
					if err != nil && err != io.EOF {
						log.Printf("Error reading from client: %v", err)
						return
					}
					if n > 0 {
						log.Printf("Sent to server: %d bytes", n)
						localToRemoteChan <- n
						_, err := remoteConn.Write(buf[:n])
						if err != nil {
							log.Printf("Error writing to remote server: %v", err)
							return
						}
						if len(listenHasActionChan) > 1 {
							log.Println("listenHasActionChan_baseConnChan count continue stop add: ", len(listenHasActionChan), len(baseConnActionChan))
							continue
						}
						actionChanStr = serverName + "_" + strconv.Itoa(n)
						baseConnActionChan <- actionChanStr
						listenHasActionChan <- actionChanStr
					}
					if err == io.EOF {
						log.Println(serverName + " client => server go func: err is io.EOF => return")
						return
					}

				}
			}()
			go func() {
				for {
					select {
					case _, ok := <-localToRemoteChan:
						// fmt.Println("byteLens, ok", byteLens, ok)
						if !ok {
							log.Println(serverName + " client => server go func select: ok is false => localToRemoteChan close")
							return
						}
						// 处理客户端数据到服务器的传输
					default:
						log.Println(serverName + " client => server go func select default: no error")
						time.Sleep(6 * time.Second)
					}
				}
			}()

			// 拦截和记录数据流量
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
			io.Copy(localConn, remoteConn)
			// +++++++++++++++++ server => client END +++++++++++++++++

		}()
		log.Println(serverName + ": go func wait")
		// 防止一直不跳出循环导致客户端无法重新连接代理
		for i := 0; i < 2; i++ {
			// for {
			select {
			case errStr = <-listenChan:
				log.Println(serverName + " listen: " + errStr)
				if errStr != "yes" {
					return errors.New(errStr)
				}
				return nil
			default:
				log.Println(serverName + " listen: no error")
				time.Sleep(1 * time.Second)
			}
		}
	}
}

func baseConnCancel(connActionChan chan string, servName string, localConn net.Listener) {

	time.Sleep(6 * time.Second)
	var baseNumInt int = 0
	log.Println("baseConnCancel localListener ready close 600s: ", servName)
	for {
		select {
		case connActionStr, ok := <-connActionChan:
			if !ok {
				localConn.Close()
				log.Println(servName + " localListener close now by connActionChan close!")
				return
			}
			baseNumInt = 0
			log.Println("baseConnCancel baseConnChan count has new value: ", connActionStr)
		default:
			time.Sleep(time.Second)
			baseNumInt++
			if baseNumInt > 600 {
				localConn.Close()
				log.Println(servName+" localListener close now! ", baseNumInt)

				return
			}
		}
	}
}

func connTimeoutCancel(ctx context.Context, cancel context.CancelFunc, serverName string, hasActionChan chan string) {

	log.Println(serverName + " ready cancel in connTimeoutCancel")
	exitChan := make(chan string, 6)
	defer close(exitChan)

	go func() {
		// 当前goroutine负责监听通道 hasActionChan
		var outInt int = 0
		for {
			select {
			case actionStr, ok := <-hasActionChan:
				if !ok {
					log.Println("hasActionChan close in connTimeoutCancel ", serverName)
					exitChan <- "exit_channel hasActionChan close"
					return
				}
				log.Printf("hasActionChan value: %s, connTimeoutCancel server -> %s \n ", actionStr, serverName)
				// 每次发送数据时间重新初始化
				outInt = 0
			default:
				time.Sleep(1 * time.Second)
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
			cancel()
			log.Println(serverName+" cancel success ++++++++ ", exitStr, " | ", ctx.Err().Error())
			return
		default:
			time.Sleep(1 * time.Second)
		}
	}
}

func connClose(ctx context.Context, serverName string, local, remote net.Conn) {
	for {
		select {
		case <-ctx.Done():
			remote.Close()
			local.Close()
			log.Println(serverName + " remoteConn localConn close now")
			return
		default:
			time.Sleep(6 * time.Second)
		}
	}
}

func sshToServer(serverName string, sshconf *confopt.SSHConfig) error {
	var (
		clientNum     int
		err           error
		errStr        string
		actionChanStr string = ""
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
		priKey, _ := PublicKeyAuth(sshconf.ServerPriKey)
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
		log.Printf("Error Failed to connect to target host: %v", err)
		return err
	}
	defer targetConn.Close()

	// 设置本地监听端口
	localListener, err := net.Listen("tcp", localHostPort)
	if err != nil {
		log.Printf("Error Failed to set up local listener: %v", err)
		return err
	}
	defer localListener.Close()

	log.Printf("Forwarding %s to %s via %s no jump", localHostPort, remoteAddr, targetHost)
	listenChan := make(chan string, 66)
	defer close(listenChan)

	// 若一定时间没有客户端接入应该主动退出重新连接
	baseConnActionChan := make(chan string, 266)
	defer close(baseConnActionChan)
	go baseConnCancel(baseConnActionChan, serverName, localListener)

	// 处理本地连接并转发
	for {
		localConn, err := localListener.Accept()
		log.Println("localListener.Accept(): ", localConn, err)
		if err != nil && strings.Contains(err.Error(), "use of closed network connection") {
			return nil
		}

		select {
		case listenChanStr, ok := <-listenChan:
			if !ok {
				log.Println("listenChan is close sshProxy restart: ")
				return nil
			}
			if listenChanStr != "yes" {
				return errors.New(listenChanStr)
			}
			return nil
		default:
			log.Println("listenChan nothing data")
		}

		if err != nil {
			// 不退出, 关闭本次链接
			log.Println("Error Failed to accept local connection: ", err)
			clientNum++
			if clientNum >= 10 {
				return err
			}
			continue
		}

		// 客户端每次发送则进行记录并告诉任务重新计时
		listenHasActionChan := make(chan string, 166)
		defer close(listenHasActionChan)

		// 用于监听客户端连接后是否在一定时间内有响应, 如果没有则需要断开
		waitCtx, waitCancel := context.WithCancel(context.Background())
		defer waitCancel()
		go connTimeoutCancel(waitCtx, waitCancel, serverName, listenHasActionChan)

		go func() {
			defer localConn.Close()

			// 建立到目标服务器的连接
			remoteConn, err := targetConn.Dial("tcp", remoteAddr)
			log.Println("go func after targetConn.Dial: ", err)
			if err != nil {
				log.Printf("Error %s Failed to connect to remote address %s: %v", serverName, remoteAddr, err)
				listenChan <- "go func targetConn.Dial error: " + err.Error()
				return
			}
			defer remoteConn.Close()

			localToRemoteChan := make(chan int, 100)
			go func() {
				defer close(localToRemoteChan)

				// 当前goroutine不能放在下面的for中, 因为Read会阻塞导致无法执行到
				go connClose(waitCtx, serverName, localConn, remoteConn)

				for {
					buf := make([]byte, 8192)
					n, err := localConn.Read(buf)
					if err != nil && err != io.EOF {
						log.Printf("Error reading from client: %v", err)
						return
					}
					if n > 0 {
						log.Printf("Sent to server: %d bytes", n)
						localToRemoteChan <- n
						_, err := remoteConn.Write(buf[:n])
						if err != nil {
							log.Printf("Error writing to remote server: %v", err)
							return
						}
						if len(listenHasActionChan) > 1 {
							log.Println("listenHasActionChan_baseConnChan count continue stop add: ", len(listenHasActionChan), len(baseConnActionChan))
							continue
						}
						actionChanStr = serverName + "_" + strconv.Itoa(n)
						baseConnActionChan <- actionChanStr
						listenHasActionChan <- actionChanStr
					}
					if err == io.EOF {
						log.Println(serverName + " client => server go func: err is io.EOF => return")
						return
					}

				}
			}()
			go func() {
				for {
					select {
					case _, ok := <-localToRemoteChan:
						if !ok {
							log.Println(serverName + " client => server go func select: ok is false => localToRemoteChan close")
							return
						}
						// 处理客户端数据到服务器的传输
					default:
						log.Println(serverName + " client => server go func select default: no error")
						time.Sleep(6 * time.Second)
					}
				}
			}()

			io.Copy(localConn, remoteConn)
		}()

		log.Println(serverName + ": go func wait")
		for i := 0; i < 2; i++ {
			select {
			case errStr = <-listenChan:
				log.Println(serverName + " listen: " + errStr)
				if errStr != "yes" {
					return errors.New(errStr)
				}
				return nil
			default:
				log.Println(serverName + " listen: no error")
				time.Sleep(1 * time.Second)
			}
		}

	}

}
