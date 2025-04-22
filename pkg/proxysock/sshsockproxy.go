package proxysock

import (
	"errors"
	"honoka/pkg/confopt"
	"log"
	"os"
	"os/exec"
	"strings"
)

func SSHSockProxy(conf *confopt.Config, onlineChan chan string) error {
	sshConf := conf.SockProxy

	// cmd := exec.Command("ssh", sshConf.SSHCommand)
	sshStr := sshConf.SSHCommand
	sshStrArr := strings.Split(sshStr, " ")
	sshArgs := sshStrArr[1:]

	cmd := exec.Command(sshStrArr[0], sshArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	log.Println("Ready run command: ", sshConf.SSHCommand)
	onlineChan <- "RunProxyServer"
	err := cmd.Run()

	// defer func() {
	// 	log.Println("Run command ssh -ND cancel now")
	// 	if cmd.Err != nil {
	// 		log.Println("Error Run command ssh -ND cancel now", cmd.Err)
	// 		return
	// 	}
	// }()

	if err != nil {
		log.Printf("Error executing command: %v\n", err)
		onlineChan <- "RestartSSHSockProxy"
		return errors.New("Error executing command: " + err.Error())
	}

	return nil
}
