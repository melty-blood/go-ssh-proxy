package proxysock

import (
	"honoka/pkg/confopt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

func UseSSHFunc(conf *confopt.Config) {
	// conf := confopt.ReadConf(confPath)

	onlineChan := make(chan string, 6)

	go RunSockToHttp(conf, onlineChan)
	RunProxyServer(conf, onlineChan)
}

func RunSockToHttp(conf *confopt.Config, onlineChan chan string) {
	if !conf.SockTOHttp.OpenStatus {
		log.Println("RunSockToHttp status false ", conf.SockTOHttp.SockAddr, conf.SockTOHttp.TOHttp)
		return
	}
	var (
		toHttpCount sync.Map
	)
	restartChan := make(chan string, 6)
	log.Println("start sock to http ", conf.SockTOHttp.SockAddr, conf.SockTOHttp.TOHttp)
	if conf.SockProxy.OpenStatus {
		go SSHSockProxy(conf, onlineChan)
	} else {
		onlineChan <- "RunProxyServer"
	}

	for {
		select {
		case online, ok := <-onlineChan:
			if !ok {
				log.Println("Error RunSockToHttp onlineChan read fail: ", conf.SockTOHttp.TOHttp)
			}
			log.Println("RunSockToHttp onlineChan value: ", online)

			if online == "RunProxyServer" {
				log.Println("RunSockToHttp SocksTOHttp start: ", conf.SockTOHttp.TOHttp)
				go StartSockToHttp(conf, &toHttpCount, restartChan)
			}

			if online == "RestartSSHSockProxy" {
				log.Println("SSHSockProxy restart")
				go SSHSockProxy(conf, onlineChan)
			}
		case restartTask, ok := <-restartChan:
			if !ok {
				log.Println("Error RunSockToHttp SocksTOHttp restart read channel fail: ", conf.SockTOHttp.TOHttp)
				restartChan <- conf.SockTOHttp.ServerName
				break
			}
			log.Println("Error RunSockToHttp SocksTOHttp restart: ", restartTask)

			go StartSockToHttp(conf, &toHttpCount, restartChan)

		default:
			time.Sleep(2 * time.Second)
		}
	}

}

func RunProxyServer(conf *confopt.Config, onlineChan chan string) {
	// conf := ReadConf(confFile)
	var (
		sshCount sync.Map
	)
	restartChan := make(chan *confopt.SSHConfig, 16)

	for _, val := range conf.ServerConf.SSHConf {
		if !val.OpenStatus {
			continue
		}
		go func(sshConf *confopt.SSHConfig) {

			log.Println("SSH ServerName go func start: ", sshConf.ServerName)
			SSHProxyStart(sshConf, conf.ServerConf.Jump, restartChan, &sshCount)
		}(val)
	}
	time.Sleep(2 * time.Second)
	log.Println("RunProxyServer channel onlineChan len: ", len(onlineChan))

	// 自己捕捉 linux 信号
	signalChanel := make(chan os.Signal, 6)
	signal.Notify(signalChanel, syscall.SIGINT, syscall.SIGTERM)
	// if runtime.GOOS == "linux"
	// signal.Notify(signalChanel, syscall.SIGINT, syscall.SIGUSR2, syscall.SIGTERM)

	var sigNum os.Signal
	for {
		select {
		case reConf, chanOk := <-restartChan:
			if reConf.IsError {
				if sshNum, ok := sshCount.Load(reConf.ServerName); ok {
					num, _ := sshNum.(int)
					log.Println("SSH restart count : ", reConf.ServerName, num)
					if num > 6 {
						log.Println("Error: This is Server connect fail many ", reConf.ServerName)
						os.Exit(888)
					}
				}
			}
			if chanOk {
				go SSHProxyStart(reConf, conf.ServerConf.Jump, restartChan, &sshCount)
			} else {
				log.Println("Error channel <-restartChan read fail: ", reConf.ServerName)
			}
		case sigNum = <-signalChanel:
			log.Println("signal number: ", sigNum)
			if sigNum == syscall.SIGTERM || sigNum == syscall.SIGINT {
				// kill -INT OR kill -TERM
				log.Println("signal number: server stop success!! +++++++++++++++++++++++ ", sigNum)
				os.Exit(222)
			} else {
				log.Println("signal number: not have signal")
			}
		default:
			time.Sleep(6 * time.Second)
		}
	}
}

func SSHProxyStart(
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
		log.Println("go func SSHProxyStart has: ", hasKey, sshConf.ServerName, sshConf.Local)
		return nil
	}
	sshCount.Store(keyName, 1)
	defer func() {
		log.Println("go func SSHProxyStart exit: ", keyName, sshConf.ServerName, sshConf.Local)
		sshCount.Delete(keyName)
	}()

	sshConf.IsError = false
	// 如果没有自定义则用公共 jump
	if sshConf.NeedJump && len(sshConf.JumpHost) == 0 {
		sshConf.JumpHost = jump.JumpHost
		sshConf.JumpUser = jump.JumpUser
		sshConf.JumpPassword = jump.JumpPassword
		sshConf.JumpPriKey = jump.JumpPriKey
	}
	log.Println("go func SSHProxyStart param ready: ", sshConf.ServerName, sshConf.Local, " - ", sshConf.JumpHost)

	// TODO 可以修改为协程使用channel来观察是否存在错误返回, 如果没有返回错误但是返回nil则代表本次连接需要被终止并重新连接
	var err error
	if sshConf.NeedJump {
		err = sshToServerByJump(sshConf.ServerName, sshConf)
	} else {
		err = sshToServer(sshConf.ServerName, sshConf)
	}
	log.Println("go func SSHProxyStart running: ", sshConf.ServerName, err)

	if err != nil {
		log.Println("Error SSHProxyStart SSH Fail: ", err, sshConf.ServerName)
		sshConf.IsError = true
		restartChan <- sshConf
	}
	restartChan <- sshConf
	return nil
}

func StartSockToHttp(conf *confopt.Config, toHttpCount *sync.Map, restartChan chan string) error {
	if toHttpCountNum, ok := toHttpCount.Load(conf.SockTOHttp.ServerName); !ok {
		toHttpCount.Store(conf.SockTOHttp.ServerName, 0)
	} else {
		sshNum, _ := toHttpCountNum.(int)
		toHttpCount.Store(conf.SockTOHttp.ServerName, sshNum+1)
	}

	log.Println("StartSockToHttp Start: ", conf.SockTOHttp.ServerName)
	err := SocksTOHttp(conf)
	// err := TempAA(conf)
	if err != nil {
		log.Println("Error StartSockToHttp SSH Fail: ", err, conf.SockTOHttp.ServerName)
		restartChan <- conf.SockTOHttp.ServerName
	}
	return err
}
