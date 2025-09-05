package proxysock

import (
	"bufio"
	"context"
	"errors"
	"honoka/pkg/confopt"
	"io"
	"log"
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
	// conf := confopt.ReadConf(confPath)

	onlineChan := make(chan string, 66)

	go RunSockToHttp(conf, onlineChan)
	RunProxySSHServer(conf, onlineChan)
}

func RunSockToHttp(conf *confopt.Config, onlineChan chan string) {
	if !conf.SockToHttp.OpenStatus {
		log.Println("RunSockToHttp status false ", conf.SockToHttp.SockAddr, conf.SockToHttp.ToHttp)
		return
	}
	var (
		toHttpCount         sync.Map
		sockCtx, sockCancel = context.WithCancel(context.Background())
	)
	defer sockCancel()
	restartChan := make(chan string, 26)
	log.Println("start sock to http ", conf.SockToHttp.SockAddr, conf.SockToHttp.ToHttp)
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
				log.Println("Error RunSockToHttp onlineChan read fail: ", conf.SockToHttp.ToHttp)
				return
			}
			log.Println("RunSockToHttp onlineChan value: ", online)

			if online == "RunProxyServer" {
				log.Println("RunSockToHttp SocksTOHttp start: ", conf.SockToHttp.ToHttp)
				go StartSockToHttp(conf, &toHttpCount, restartChan)
			}

			if online == "RestartSSHSockProxy" {
				log.Println("SSHSockProxy restart")
				sockCtx, sockCancel := context.WithCancel(context.Background())
				defer sockCancel()
				go RunSSHSock5(sockCtx, conf, onlineChan)
			}
		case restartTask, ok := <-restartChan:
			if !ok {
				log.Println("Error RunSockToHttp SocksTOHttp restart read channel fail: ", conf.SockToHttp.ToHttp)
				restartChan <- conf.SockToHttp.ServerName
				break
			}
			log.Println("Error RunSockToHttp SocksTOHttp restart: ", restartTask)
			go StartSockToHttp(conf, &toHttpCount, restartChan)

		case sigNum := <-signalChannel:
			log.Println("RunSockToHttp->signal number: ", sigNum)
			if sigNum == sigUSR2 {
				log.Println("signal number: syscall.SIGUSR2!! +++++++++++++++++++++++ ", sigNum)
				sockCancel()
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

			log.Println("RunProxySSHServer SSH ServerName go func start: ", sshConf.ServerName)
			SSHProxyStart(ctx, sshConf, conf.ServerConf.Jump, restartChan, &sshCount)
		}(val)
	}
	time.Sleep(2 * time.Second)
	log.Println("RunProxySSHServer RunProxyServer channel onlineChan len: ", len(onlineChan))

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
					log.Println("RunProxySSHServer SSH restart count : ", reConf.ServerName, num)
					if num > 6 {
						log.Println("RunProxySSHServer Error: This is Server connect fail many ", reConf.ServerName)
						os.Exit(888)
					}
				}
			}
			if chanOk {
				if runSSHServer, ok := serverSSHMap[reConf.ServerName]; ok {
					loadVal, loadOk := sshCount.Load("key_" + reConf.ServerName)
					log.Println("RunProxySSHServer <-restartChan Load: ", loadVal, loadOk)
					// 原本的cancel需要取消, 然后在赋值新的
					runSSHServer.ctxCancel()
					// 等待1秒, 等待 SSHProxyStart 清理工作
					time.Sleep(1 * time.Second)
					loadVal, loadOk = sshCount.Load("key_" + reConf.ServerName)
					log.Println("RunProxySSHServer <-restartChan Load after 2: ", loadVal, loadOk)

					ctx, cancel := context.WithCancel(context.Background())
					runSSHServer.ctx = ctx
					runSSHServer.ctxCancel = cancel
					go SSHProxyStart(ctx, reConf, conf.ServerConf.Jump, restartChan, &sshCount)
				}
			} else {
				log.Println("RunProxySSHServer Error channel <-restartChan read fail: ", reConf.ServerName)
			}
		case sigNum = <-signalChannel:
			log.Println("RunProxySSHServer->signal number: ", sigNum)
			if sigNum == syscall.SIGTERM || sigNum == syscall.SIGINT {
				// kill -INT OR kill -TERM
				log.Println("RunProxySSHServer signal number: server stop success!!", sigNum)
				os.Exit(222)
			}
			if sigNum == sigUSR1 {
				log.Println("RunProxySSHServer signal number: syscall.SIGUSR1!!", sigNum)

				err = reloadSSHProxy(conf.ServerConf.SignalOrderFilePath, serverSSHMap)
				if err != nil {
					log.Println("RunProxySSHServer->readOrderBySignal Error: ", err)
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

	// 幂等 每次只能有一组在运行
	keyName := "key_" + sshConf.ServerName
	if hasKey, ok := sshCount.Load(keyName); ok {
		log.Println("SSHProxyStart has:", hasKey, sshConf.ServerName, sshConf.Local)
		return nil
	}
	sshCount.Store(keyName, 1)
	defer func() {
		log.Println("SSHProxyStart exit:", keyName, sshConf.ServerName, sshConf.Local)
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
	}
	log.Println("SSHProxyStart param ready:", sshConf.ServerName, sshConf.Local, " - ", sshConf.JumpHost)

	// TODO 可以修改为协程使用channel来观察是否存在错误返回, 如果没有返回错误但是返回nil则代表本次连接需要被终止并重新连接
	var err error
	if sshConf.NeedJump {
		err = sshToServerByJump(ctx, sshConf.ServerName, sshConf)
	} else {
		err = sshToServer(ctx, sshConf.ServerName, sshConf)
	}
	log.Println("SSHProxyStart run over:", sshConf.ServerName, err)

	if err != nil {
		log.Println("SSHProxyStart SSH Fail:", err, sshConf.ServerName)
		sshConf.IsError = true
		return err
	}
	return nil
}

func StartSockToHttp(conf *confopt.Config, toHttpCount *sync.Map, restartChan chan string) error {
	keyName := "key_" + conf.SockToHttp.ServerName
	if hasKey, ok := toHttpCount.Load(keyName); ok {
		log.Println("StartSockToHttp has:", hasKey, keyName)
		return nil
	}
	toHttpCount.Store(keyName, 1)
	defer func() {
		log.Println("StartSockToHttp exit:", keyName)
		toHttpCount.Delete(keyName)
	}()

	if toHttpCountNum, ok := toHttpCount.Load(conf.SockToHttp.ServerName); !ok {
		toHttpCount.Store(conf.SockToHttp.ServerName, 0)
	} else {
		sshNum, _ := toHttpCountNum.(int)
		toHttpCount.Store(conf.SockToHttp.ServerName, sshNum+1)
	}

	log.Println("StartSockToHttp Start: ", conf.SockToHttp.ServerName)
	err := SocksToHttps(conf)
	if err != nil {
		log.Println("Error StartSockToHttp SSH Fail: ", err, conf.SockToHttp.ServerName)
		restartChan <- conf.SockToHttp.ServerName
		return err
	}
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
	orderSSHMap, err := readOrderBySignal(orderPath)
	if err != nil {
		return err
	}
	if orderSSHOne, ok := orderSSHMap[OrderSSHProxyReloadOne]; ok {
		orderSSHOneArr := strings.Split(orderSSHOne, ",")
		for _, val := range orderSSHOneArr {
			if serverSSHOne, ok := serverSSHMap[val]; ok {
				serverSSHOne.ctxCancel()
				log.Println("reloadSSHProxy ctxCancel by:", serverSSHOne.serverName)
			}
		}
	}
	return nil
}
