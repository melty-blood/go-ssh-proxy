package sshcmd

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

type (
	SSHConf struct {
		User, Password, IdentityFile, Host, Port string
	}
	SftpFile struct {
		LFilePath, RFilePath string
	}
)

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

func SSHConnect(conf *SSHConf) (*ssh.Client, error) {
	if len(conf.Password) <= 0 && len(conf.IdentityFile) <= 0 {
		return nil, errors.New("must has password or identity file")
	}

	sshAuth := []ssh.AuthMethod{ssh.Password(conf.Password)}
	if len(conf.IdentityFile) > 0 {
		priKey, err := PublicKeyAuth(conf.IdentityFile)
		if err != nil {

		}
		sshAuth = []ssh.AuthMethod{priKey}
	}

	config := &ssh.ClientConfig{
		User:            conf.User,
		Auth:            sshAuth,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         5 * time.Second,
	}

	addr := fmt.Sprintf("%s:%s", conf.Host, conf.Port)
	return ssh.Dial("tcp", addr, config)
}

func RunCmdWithSSH(cmdList []string, conn *ssh.Client) ([]string, error) {
	var (
		err error
		// cmdOutByte []byte
		cmdOutList []string
		buf        bytes.Buffer
	)

	session, err := conn.NewSession()
	if err != nil {
		return cmdOutList, fmt.Errorf("create SSH session failed: %w", err)
	}
	defer session.Close()

	stdin, err := session.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := session.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr, err := session.StderrPipe()
	if err != nil {
		return nil, err
	}
	if err := session.Shell(); err != nil {
		return nil, err
	}

	pipeRead, pipeWrite := io.Pipe()
	defer func() {
		pipeRead.Close()
		pipeWrite.Close()
	}()
	var wg sync.WaitGroup
	wg.Add(2)
	// stdout -> pipeWrite
	go func() {
		defer wg.Done()
		copyBuf := make([]byte, 8388608)
		_, _ = io.CopyBuffer(pipeWrite, stdout, copyBuf)
	}()
	// stderr -> pipeWrite
	go func() {
		defer wg.Done()
		copyBuf := make([]byte, 8388608)
		_, _ = io.CopyBuffer(pipeWrite, stderr, copyBuf)
	}()
	// reader := bufio.NewReaderSize(io.MultiReader(stdout, stderr), 8388608)
	reader := bufio.NewReaderSize(pipeRead, 8388608)
	cmdSep := "__CMDEND__"

	for _, cmd := range cmdList {
		fmt.Fprintf(stdin, "echo '$ %s'\n", cmd)
		fmt.Fprintln(stdin, cmd)
		fmt.Fprintf(stdin, "echo %s\n", cmdSep)

		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				return cmdOutList, fmt.Errorf("read output: %w", err)
			}
			if strings.Contains(line, cmdSep) {
				break
			}
			buf.WriteString(line)
		}
		cmdOutList = append(cmdOutList, buf.String())
		buf.Reset()
	}

	fmt.Fprintln(stdin, "exit")
	stdin.Close()

	// stdIn, _ := session.StdinPipe()
	// var buf bytes.Buffer
	// session.Stdout = &buf
	// session.Stderr = &buf
	// // session.Stdout = os.Stdout
	// // session.Stderr = os.Stdout
	// if err := session.Shell(); err != nil {
	// 	return cmdOutList, fmt.Errorf("create SSH session shell failed: %w", err)
	// }

	// for _, cmdV := range cmdList {
	// 	fmt.Fprintln(stdIn, cmdV)
	// 	fmt.Fprintln(stdIn, "echo __CMDEND__")
	// 	time.Sleep(1200 * time.Millisecond)
	// }

	// fmt.Fprintln(stdIn, "exit")
	// stdIn.Close()
	if err = session.Wait(); err != nil && err != io.EOF {
		return cmdOutList, fmt.Errorf("SSH session shell wait failed: %w", err)
	}

	// cmdOut := buf.String()
	// fmt.Println("cmdOut: ", cmdOut)

	// fmt.Println("cmdOut: ", len(cmdOutList), cmdOutList)
	// fmt.Println("++++++++++++++++++++++ ", cmdOutList[0])
	return cmdOutList, err
}

// sftp
func SSHUploadFile(fileList []*SftpFile, conn *ssh.Client) error {
	var (
		err   error
		rFile *sftp.File
		lFile *os.File
	)

	sftpClient, err := sftp.NewClient(conn)
	if err != nil {
		return errors.New("sftp NewClient failed: " + err.Error())
	}
	defer sftpClient.Close()

	for _, vFile := range fileList {
		lFile, err = os.OpenFile(vFile.LFilePath, os.O_RDONLY, 0)
		if err != nil {
			return errors.New("open local file failed: " + err.Error())
		}

		rFile, err = sftpClient.Create(vFile.RFilePath)
		if err != nil {
			return errors.New("sftp create remote file failed: " + err.Error())
		}
		_, err = rFile.ReadFrom(lFile)
		if err != nil {
			return errors.New("sftp upload file failed: " + err.Error())
		}
		lFile.Close()
		rFile.Close()
	}
	return nil
}
