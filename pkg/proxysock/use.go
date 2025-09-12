package proxysock

import (
	"bufio"
	"context"
	"errors"
	"honoka/pkg/confopt"
	"io"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"
)

type runSSHServer struct {
	ctx        context.Context
	ctxCancel  context.CancelFunc
	serverName string
}

const (
	OrderSSHProxyReloadOne = "OrderSSHProxyReloadOne"
)

func UseSSHFunc(conf *confopt.Config) {
	go func() {
		http.ListenAndServe("127.0.0.1:6060", nil)
	}()

	onlineChan := make(chan string, 66)
	go RunSockToHttp(conf, onlineChan)
	RunProxySSHServer(conf, onlineChan)
}

func RunSockToHttp(conf *confopt.Config, onlineChan chan string) {
	logp := NewPrintLog("RunSockToHttp", "")
	if !conf.SockToHttp.OpenStatus {
		logp.Print("status false ", conf.SockToHttp.SockAddr, conf.SockToHttp.ToHttp)
		return
	}
	var (
		toHttpCount         sync.Map
		sockCtx, sockCancel = context.WithCancel(context.Background())
		sockMap             = make(map[string]*runSSHServer)
	)
	defer sockCancel()

	sockMap[conf.SockToHttp.ServerName] = &runSSHServer{
		ctx:        sockCtx,
		ctxCancel:  sockCancel,
		serverName: conf.SockToHttp.ServerName,
	}

	restartChan := make(chan string, 26)
	logp.Print("start socks5 to http:", conf.SockToHttp.SockAddr, conf.SockToHttp.ToHttp)
	if conf.SockProxy.OpenStatus {
		go RunSSHSock5(sockCtx, conf, onlineChan)
	} else {
		onlineChan <- "RunProxyServer"
	}

	// 获取 linux 信号
	signalChannel := make(chan os.Signal, 6)
	signal.Notify(signalChannel, sigUSR1, sigUSR2)

	for {
		select {
		case online, ok := <-onlineChan:
			if !ok {
				logp.Print("onlineChan read close:", conf.SockToHttp.ToHttp)
				return
			}
			logp.Print("received onlineChan-> value:", online)

			if online == "RunProxyServer" {
				logp.Print("channel received, StartSockToHttp start:", conf.SockToHttp.ToHttp)
				go StartSockToHttp(conf, &toHttpCount, restartChan)
			}

			if online == "RestartSSHSockProxy" {
				logp.Print("SSHSockProxy restart")
				sockCtx, sockCancel := context.WithCancel(context.Background())
				sockMap[conf.SockToHttp.ServerName].ctx = sockCtx
				sockMap[conf.SockToHttp.ServerName].ctxCancel = sockCancel

				go RunSSHSock5(sockCtx, conf, onlineChan)
			}
		case restartTask, ok := <-restartChan:
			if !ok {
				logp.Print("Error SocksTOHttp restart read channel close:", conf.SockToHttp.ToHttp)
				restartChan <- conf.SockToHttp.ServerName
				break
			}
			logp.Print("channel restart StartSockToHttp:", restartTask)
			go StartSockToHttp(conf, &toHttpCount, restartChan)

		case sigNum := <-signalChannel:
			logp.Print("received signal value:", sigNum)
			if sigNum == sigUSR2 {
				logp.Print("signal number: syscall.SIGUSR2", sigNum)
				sockMap[conf.SockToHttp.ServerName].ctxCancel()

				go func() {
					time.Sleep(time.Second * 6)
					onlineChan <- "RestartSSHSockProxy"
				}()
			}

		default:
			time.Sleep(2 * time.Second)
		}
	}
}

func RunProxySSHServer(conf *confopt.Config, onlineChan chan string) {
	var (
		err              error
		sshCount         sync.Map
		serverSSHMap     = make(map[string]*runSSHServer, 66)
		serverSSHMapLock sync.Mutex
		logp             = NewPrintLog("RunProxySSHServer", "")
	)
	restartChan := make(chan *confopt.SSHConfig, 16)

	for _, val := range conf.ServerConf.SSHConf {
		if !val.OpenStatus {
			continue
		}
		go func(sshConf *confopt.SSHConfig) {
			serverSSHMapLock.Lock()
			// ctx, cancel不能使用 var 提前声明, 否则会造成 ctx, cancel混乱,
			// 导致关闭其他的 SSHServer 代理, 无法达到关闭预期的 SSHServer
			ctx, cancel := context.WithCancel(context.Background())
			serverSSHMap[sshConf.ServerName] = &runSSHServer{
				ctx:        ctx,
				ctxCancel:  cancel,
				serverName: sshConf.ServerName,
			}
			serverSSHMapLock.Unlock()

			logp.Print("go func SSHProxyStart start:", sshConf.ServerName)
			SSHProxyStart(ctx, sshConf, conf.ServerConf.Jump, restartChan, &sshCount)
		}(val)
	}
	time.Sleep(2 * time.Second)
	logp.Print("channel onlineChan len:", len(onlineChan))

	// 获取 linux 信号
	signalChannel := make(chan os.Signal, 6)
	signal.Notify(signalChannel, syscall.SIGINT, syscall.SIGTERM, sigUSR1)

	var sigNum os.Signal
	for {
		select {
		case reConf, chanOk := <-restartChan:
			if reConf.IsError {
				if sshNum, ok := sshCount.Load(reConf.ServerName); ok {
					num, _ := sshNum.(int)
					logp.Print("SSH proxy restart count:", reConf.ServerName, num)
					if num > 6 {
						logp.Print("Error: This is SSH Server connect fail too many", reConf.ServerName)
						os.Exit(888)
					}
				}
			}
			if chanOk {
				if runSSHServer, ok := serverSSHMap[reConf.ServerName]; ok {
					loadVal, loadOk := sshCount.Load("key_" + reConf.ServerName)
					logp.Print("<-restartChan sshCount load:", reConf.ServerName, loadVal, loadOk)
					// 原本的cancel需要取消, 然后在赋值新的
					runSSHServer.ctxCancel()
					// 等待1秒, 等待 SSHProxyStart 清理工作
					time.Sleep(1 * time.Second)
					loadVal, loadOk = sshCount.Load("key_" + reConf.ServerName)
					logp.Print("<-restartChan sshCount load 2:", reConf.ServerName, loadVal, loadOk)

					ctx, cancel := context.WithCancel(context.Background())
					runSSHServer.ctx = ctx
					runSSHServer.ctxCancel = cancel
					go SSHProxyStart(ctx, reConf, conf.ServerConf.Jump, restartChan, &sshCount)
				}
			} else {
				logp.Print("Error channel <-restartChan read close:", reConf.ServerName)
				os.Exit(666)
			}
		case sigNum = <-signalChannel:
			logp.Print("->signal number:", sigNum)
			if sigNum == syscall.SIGTERM || sigNum == syscall.SIGINT {
				// kill -INT OR kill -TERM
				logp.Print("signal number: server stop success!!", sigNum)
				os.Exit(222)
			}
			if sigNum == sigUSR1 {
				logp.Print("signal number: syscall.SIGUSR1!!", sigNum)

				err = reloadSSHProxy(conf.ServerConf.SignalOrderFilePath, serverSSHMap)
				if err != nil {
					logp.Print("->readOrderBySignal Error:", err)
				}
			}

		default:
			time.Sleep(6 * time.Second)
		}
	}
}

func SSHProxyStart(
	ctx context.Context,
	sshConf *confopt.SSHConfig,
	jump *confopt.CommonJump,
	restartChan chan *confopt.SSHConfig,
	sshCount *sync.Map,
) error {
	if sshCountNum, ok := sshCount.Load(sshConf.ServerName); !ok {
		sshCount.Store(sshConf.ServerName, 0)
	} else {
		sshNum, _ := sshCountNum.(int)
		sshCount.Store(sshConf.ServerName, sshNum+1)
	}
	logp := NewPrintLog("SSHProxyStart", "")

	// 幂等 每次只能有一组在运行
	keyName := "key_" + sshConf.ServerName
	if hasKey, ok := sshCount.Load(keyName); ok {
		logp.Print("sshCount.Load has:", hasKey, keyName, sshConf.Local)
		return nil
	}
	sshCount.Store(keyName, 1)
	defer func() {
		logp.Print("SSH proxy stop defer exit:", keyName, sshConf.ServerName, sshConf.Local)
		sshCount.Delete(keyName)
		restartChan <- sshConf
	}()

	sshConf.IsError = false
	// 如果没有自定义则用公共 jump
	if sshConf.NeedJump && len(sshConf.JumpHost) == 0 {
		sshConf.JumpHost = jump.JumpHost
		sshConf.JumpUser = jump.JumpUser
		sshConf.JumpPassword = jump.JumpPassword
		sshConf.JumpPriKey = jump.JumpPriKey
		sshConf.JumpPriPass = jump.JumpPriPass
	}
	logp.Print("param ready:", sshConf.ServerName, sshConf.Local, " - ", sshConf.JumpHost)

	// TODO 可以修改为协程使用channel来观察是否存在错误返回, 如果没有返回错误但是返回nil则代表本次连接需要被终止并重新连接
	var err error
	if sshConf.NeedJump {
		err = sshToServerByJump(ctx, sshConf.ServerName, sshConf)
	} else {
		err = sshToServer(ctx, sshConf.ServerName, sshConf)
	}
	logp.Print("process run over:", sshConf.ServerName, err)

	if err != nil {
		logp.Print("SSH server process Fail:", err, sshConf.ServerName)
		sshConf.IsError = true
		return err
	}
	return nil
}

func StartSockToHttp(conf *confopt.Config, toHttpCount *sync.Map, restartChan chan string) error {
	logp := NewPrintLog("StartSockToHttp", "")

	keyName := "key_" + conf.SockToHttp.ServerName
	if hasKey, ok := toHttpCount.Load(keyName); ok {
		logp.Print("toHttpCount.Load has:", hasKey, keyName)
		return nil
	}
	toHttpCount.Store(keyName, 1)
	defer func() {
		logp.Print("sock to http stop defer exit:", keyName)
		toHttpCount.Delete(keyName)
	}()

	if toHttpCountNum, ok := toHttpCount.Load(conf.SockToHttp.ServerName); !ok {
		toHttpCount.Store(conf.SockToHttp.ServerName, 0)
	} else {
		sshNum, _ := toHttpCountNum.(int)
		toHttpCount.Store(conf.SockToHttp.ServerName, sshNum+1)
	}

	logp.Print("sock to http process Start:", conf.SockToHttp.ServerName)
	err := SocksToHttps(conf)
	if err != nil {
		logp.Print("Error socks proxy http(s) Fail:", err, conf.SockToHttp.ServerName)
		restartChan <- conf.SockToHttp.ServerName
		return err
	}
	logp.Print("sock to http process run end:", conf.SockToHttp.ServerName)
	return nil
}

func readOrderBySignal(filePath string) (map[string]string, error) {
	file, err := os.OpenFile(filePath, os.O_RDONLY, 0)
	if err != nil {
		return nil, err
	}
	proxySSHMap := make(map[string]string, 166)
	scan := bufio.NewReaderSize(file, 8388608)
	for {
		line, _, err := scan.ReadLine()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, errors.New("scnner readLine error: " + err.Error())
		}
		proxyConfOne := strings.Split(string(line), ":")
		if len(proxyConfOne) <= 1 {
			continue
		}
		proxySSHMap[proxyConfOne[0]] = proxyConfOne[1]
	}
	return proxySSHMap, nil
}

func reloadSSHProxy(orderPath string, serverSSHMap map[string]*runSSHServer) error {
	logp := NewPrintLog("reloadSSHProxy", "")

	orderSSHMap, err := readOrderBySignal(orderPath)
	if err != nil {
		return err
	}
	if orderSSHOne, ok := orderSSHMap[OrderSSHProxyReloadOne]; ok {
		orderSSHOneArr := strings.Split(orderSSHOne, ",")
		for _, val := range orderSSHOneArr {
			if serverSSHOne, ok := serverSSHMap[val]; ok {
				serverSSHOne.ctxCancel()
				logp.Print("ctxCancel by:", serverSSHOne.serverName)
			}
		}
	}
	return nil
}
