package svc

import (
	"bufio"
	"errors"
	"fmt"
	"honoka/pkg/confopt"
	"honoka/pkg/network/sshcmd"
	"honoka/pkg/tarzip"
	"log"
	"os"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/plumbing"
	"github.com/go-git/go-git/v6/plumbing/object"
	"github.com/go-git/go-git/v6/plumbing/transport/http"
	"github.com/go-git/go-git/v6/plumbing/transport/ssh"
)

const (
	envName     = "${ENV_NAME}"
	sshEnvPath  = "${SSH_ENV_PATH}"
	uploadPath  = "${UPLOAD_PATH}"
	packageName = "${PACKAGE_NAME}"
	packagePath = "${PACKAGE_PATH}"
)

func InitEnvParam(envArr []string) map[string]string {
	var (
		envKV []string
	)
	envMap := map[string]string{
		envName:     "ssh_env_path_replace",
		sshEnvPath:  "ssh_env_path_replace",
		uploadPath:  "package_name_replace",
		packageName: "package_name_replace",
		packagePath: "package_name_replace",
	}
	for _, env := range envArr {
		envKV = strings.Split(env, "|")
		if len(envKV) < 2 {
			continue
		}
		if _, ok := envMap[envKV[0]]; ok {
			envMap[envKV[0]] = envKV[1]
		}
	}
	return envMap
}

type (
	PublishFastOrder struct {
		GitKey, GitEnv string
	}
)

func PublishSSH(gitConf *confopt.PublishGitOpt) error {
	// os.Remove()
	var (
		err         error
		publishPath string
	)

	for _, envV := range gitConf.EnvList {
		if envV.EnvNum == gitConf.SelectEnv {
			publishPath = envV.EnvPath
			break
		}
	}
	if len(publishPath) <= 0 {
		return errors.New("publishPath is empty, please check select env")
	}
	// last string is '/'
	if publishPath[len(publishPath)-1:] != string(os.PathSeparator) {
		publishPath = publishPath + string(os.PathSeparator)
	}

	err = tarzip.CreateTargz(gitConf.ClonePath, gitConf.TargzPath, gitConf.TargzIsNeedTopDir)
	if err != nil {
		return err
	}
	uploadNameArr := strings.Split(gitConf.SftpUploadPath, string(os.PathSeparator))
	packageTargzName := uploadNameArr[len(uploadNameArr)-1]
	fileName := publishPath + packageTargzName

	// init ENV_MAP
	envArr := []string{
		envName + "|" + gitConf.SelectEnv,
		sshEnvPath + "|" + publishPath,
		uploadPath + "|" + gitConf.SftpUploadPath,
		packageName + "|" + packageTargzName,
		packagePath + "|" + fileName,
	}
	envMap := InitEnvParam(envArr)

	// if target is multiple machines
	if gitConf.IsSSHCluster {
		err = runSSHCluster(gitConf, envMap)
		if err != nil {
			return err
		}
		return nil
	}

	// ready excute command
	newCmd := replaceSSHCmd(gitConf.SSHCmd, envMap)
	sshConf := &sshcmd.SSHConf{
		User:         gitConf.SSHUser,
		Password:     gitConf.SSHPasswd,
		IdentityFile: gitConf.SSHIdentityFile,
		Host:         gitConf.SSHHost,
		Port:         gitConf.SSHPort,
	}
	// upload file params
	fileMap := []*sshcmd.SftpFile{
		{
			LFilePath: gitConf.TargzPath,
			RFilePath: gitConf.SftpUploadPath,
		},
	}
	cmdRes, err := runSSH(sshConf, fileMap, newCmd)
	if err != nil {
		return err
	}
	if gitConf.IsShowSSHCmdOut {
		fmt.Println("============ CMD OUTPUT ============")
		for _, cmdV := range cmdRes {
			fmt.Println(strings.Trim(cmdV, ""))
		}
	}
	return nil
}

func PublishFastOrderGit(fastOrder *PublishFastOrder, conf *confopt.Config) (*confopt.PublishGitOpt, error) {
	gitMap, _ := buildPublishMap(conf)
	confGitInfo, ok := gitMap[fastOrder.GitKey]
	if !ok {
		return nil, errors.New("error git key not exists in conf file")
	}
	confGitInfo.SelectEnv = fastOrder.GitEnv
	log.Println("fastOrder:", fastOrder.GitKey, "--", fastOrder.GitEnv)

	return PublishCode(confGitInfo)
}

func PublishInteractionGit(conf *confopt.Config) (*confopt.PublishGitOpt, error) {
	var (
		selectGitInput, selectEnvInput string
		envList                        []string
	)
	gitMap, gitkeyArr := buildPublishMap(conf)

	// select git name
	fmt.Printf("Select you git key? (%s):", strings.Join(gitkeyArr, ","))

	selectGitScan := bufio.NewScanner(os.Stdin)
	if selectGitScan.Scan() {
		selectGitInput = selectGitScan.Text()
		selectGitInput = strings.ToLower(strings.TrimSpace(selectGitInput))
		if !slices.Contains(gitkeyArr, selectGitInput) {
			return nil, errors.New("input git key not exists in conf")
		}
	}
	fmt.Println("selectGitInput:", selectGitInput)

	confGitInfo := gitMap[selectGitInput]
	// select env
	for _, val := range confGitInfo.EnvList {
		envList = append(envList, val.EnvNum)
	}
	fmt.Printf("Select you git env? (%s):", strings.Join(envList, ","))

	selectEnvScan := bufio.NewScanner(os.Stdin)
	if selectEnvScan.Scan() {
		selectEnvInput = selectEnvScan.Text()
		selectEnvInput = strings.ToLower(strings.TrimSpace(selectEnvInput))

		if !slices.Contains(envList, selectEnvInput) {
			return nil, errors.New("input env not exists in conf")
		}
	}
	confGitInfo.SelectEnv = selectEnvInput
	fmt.Println("selectEnvInput:", selectEnvInput)

	return PublishCode(confGitInfo)
}

func PublishCode(opt *confopt.PublishGitOpt) (*confopt.PublishGitOpt, error) {
	// init clone path directory
	clonePathArr := strings.Split(opt.ClonePath, "/")
	err := os.Mkdir(strings.Join(clonePathArr[:len(clonePathArr)-1], "/"), 0755)
	if err != nil && !os.IsExist(err) {
		return nil, errors.New("clone path create fail" + err.Error())
	}

	// git clone
	gitRep, err := git.PlainOpenWithOptions(opt.ClonePath, &git.PlainOpenOptions{})
	if err != nil && errors.Is(err, git.ErrRepositoryNotExists) {
		log.Println("ready clone remote:", err)
		gitRep, err = GitCloneSSH(opt)
		if err != nil {
			return nil, errors.New("git clone failed:" + err.Error())
		}
	} else if err != nil {
		return nil, errors.New("git read repo failed:" + err.Error())
	}

	// git checkout
	remoteRefName := plumbing.NewRemoteReferenceName(opt.RemoteName, opt.RemoteBranch)
	remoteRef, err := gitRep.Reference(remoteRefName, true) // true表示解析符号引用
	if err != nil {
		return nil, errors.New("can not found remote branch:" + err.Error())
	}
	remoteCommitHash := remoteRef.Hash()
	log.Printf("Find remote branche: '%s', point at Commit: %s \n", remoteRefName.Short(), remoteCommitHash)

	// show HEAD
	gitReference, err := gitRep.Head()
	if err != nil {
		return nil, errors.New("git show Head failed:" + err.Error())
	}
	workTree, err := gitRep.Worktree()
	if gitReference.Name().Short() != opt.CheckBranch {
		log.Println("gitRep.workTree: ", err)
		err = workTree.Checkout(&git.CheckoutOptions{
			// Hash: new branch point to this Commit
			Hash:   remoteCommitHash,
			Branch: plumbing.NewBranchReferenceName(opt.CheckBranch),
			Create: true,
		})
		if err != nil {
			return nil, errors.New("git checkout branch failed:" + err.Error())
		}
	}
	log.Println("git pull head name:", gitReference.Name().Short())
	err = GitPullCode(gitRep, workTree, opt)
	if err != nil {
		return nil, errors.New("git pull code failed:" + err.Error())
	}

	// show logs
	logsArr, err := GetBranchLog(gitRep, 2)
	if err != nil {
		return nil, errors.New("git logs err:" + err.Error())
	}
	for _, logsV := range logsArr {
		fmt.Println("show git log: ", logsV)
	}
	return opt, nil
}

func GitCloneSSH(opt *confopt.PublishGitOpt) (*git.Repository, error) {
	pubKeys, err := ssh.NewPublicKeysFromFile(opt.SSHGitUser, opt.SSHGitIdentityFile, opt.SSHGitIdentityPasswd)
	if err != nil {
		return nil, errors.New("GitCloneSSH priKey failed: " + err.Error())
	}

	return git.PlainClone(opt.ClonePath, &git.CloneOptions{
		URL:      opt.RepoUrl,
		Progress: os.Stdout,
		Auth:     pubKeys,
	})
}

func GitCloenHttp(opt *confopt.PublishGitOpt) (*git.Repository, error) {
	return git.PlainClone(opt.ClonePath, &git.CloneOptions{
		URL:      opt.RepoUrl,
		Progress: os.Stdout,
		Auth: &http.BasicAuth{
			Username: opt.HttpsGitUser,
			Password: opt.HttpsGitPat,
		},
	})
}

func GitPullCode(gitRep *git.Repository, workTree *git.Worktree, opt *confopt.PublishGitOpt) error {
	head, err := gitRep.Head()
	if err != nil {
		return err
	}
	pullOpt := &git.PullOptions{
		RemoteName:    opt.RemoteName,
		ReferenceName: plumbing.NewBranchReferenceName(head.Name().Short()),
		SingleBranch:  true,
		Auth: &http.BasicAuth{
			Username: opt.HttpsGitUser,
			Password: opt.HttpsGitPat,
		},
		Progress: os.Stdout,
	}
	if len(opt.SSHGitIdentityFile) > 0 {
		pubKeys, err := ssh.NewPublicKeysFromFile(opt.SSHGitUser, opt.SSHGitIdentityFile, opt.SSHGitIdentityPasswd)
		if err != nil {
			return errors.New("GitPull priKey failed: " + err.Error())
		}
		pullOpt.Auth = pubKeys
	}

	err = workTree.Pull(pullOpt)
	if err == git.NoErrAlreadyUpToDate {
		log.Println("git pull err NoErrAlreadyUpToDate:", err)
		return nil
	}
	if err != nil {
		return errors.New("git pull err:" + err.Error())
	}
	return nil
}

func GetBranchLog(gitRep *git.Repository, limit int) ([]string, error) {
	logsAdd := []string{}
	head, err := gitRep.Head()
	if err != nil {
		return logsAdd, err
	}

	cnt := 0
	cIter, err := gitRep.Log(&git.LogOptions{From: head.Hash()})
	if err != nil {
		return logsAdd, err
	}
	defer cIter.Close()

	cIter.ForEach(func(c *object.Commit) error {
		if limit > 0 && cnt >= limit {
			return nil
		}
		logsAdd = append(logsAdd, c.Hash.String()+" | "+c.Author.When.Format(time.DateTime)+" | "+c.Message)
		cnt++
		return nil
	})

	return logsAdd, nil
}

func buildPublishMap(conf *confopt.Config) (gitMap map[string]*confopt.PublishGitOpt, gitArr []string) {
	gitMap = map[string]*confopt.PublishGitOpt{}

	for _, val := range conf.Publish.GitList {
		gitMap[val.KeyName] = val
		gitArr = append(gitArr, val.KeyName)
	}
	return
}

func runSSHCluster(gitConf *confopt.PublishGitOpt, envMap map[string]string) error {
	if len(gitConf.SSHCluster) < 1 {
		return errors.New("SSHCluster conf nothing, check conf")
	}
	var (
		sshConf  *sshcmd.SSHConf
		wgRunSSH sync.WaitGroup
	)
	errSSHMap := map[string]string{}
	fileMap := []*sshcmd.SftpFile{
		{
			LFilePath: gitConf.TargzPath,
			RFilePath: gitConf.SftpUploadPath,
		},
	}
	sshShareCmd := replaceSSHCmd(gitConf.SSHCmd, envMap)
	cmdOutputMap := map[string][]string{}

	wgRunSSH.Add(len(gitConf.SSHCluster))
	for _, cVal := range gitConf.SSHCluster {

		go func() {
			defer wgRunSSH.Done()
			log.Println("start ssh ->", cVal.SSHHost)

			sshCmd := sshShareCmd
			// if false, use self diy cmd, true use parent cmd
			if !cVal.IsUseParentCmd {
				sshCmd = replaceSSHCmd(cVal.SSHCmd, envMap)
			}
			sshConf = &sshcmd.SSHConf{
				User:         cVal.SSHUser,
				Password:     cVal.SSHPasswd,
				IdentityFile: cVal.SSHIdentityFile,
				Host:         cVal.SSHHost,
				Port:         cVal.SSHPort,
			}
			cmdOutput, err := runSSH(sshConf, fileMap, sshCmd)
			if err != nil {
				errSSHMap[sshConf.Host] = err.Error()
			}
			if cVal.IsShowSSHCmdOut {
				cmdOutputMap[cVal.SSHHost] = cmdOutput
			}
		}()
	}
	wgRunSSH.Wait()
	if len(errSSHMap) > 0 {
		for eKey, eVal := range errSSHMap {
			log.Println("Error: run ssh has err:", eKey, " ==> ", eVal)
		}
		return errors.New("run ssh cluster has some error, see the above log")
	}

	for oKey, oVal := range cmdOutputMap {
		fmt.Println("============ CMD OUTPUT ", oKey, "============")
		for _, cmdRes := range oVal {
			fmt.Println(strings.Trim(cmdRes, ""))
		}
	}
	return nil
}

func runSSH(sshConf *sshcmd.SSHConf, sshUploadFile []*sshcmd.SftpFile, sshCmd []string) ([]string, error) {
	conn, err := sshcmd.SSHConnect(sshConf)
	if err != nil {
		return nil, errors.New("ssh connect err:" + err.Error())
	}
	// upload file to server
	err = sshcmd.SSHUploadFile(sshUploadFile, conn)
	if err != nil {
		return nil, errors.New("ssh upload fail:" + err.Error())
	}
	cmdRes, err := sshcmd.RunCmdWithSSH(sshCmd, conn)
	if err != nil {
		return nil, errors.New("ssh run cmd fail:" + err.Error())
	}

	return cmdRes, nil
}

func replaceSSHCmd(cmd []string, envMap map[string]string) []string {
	newCmd := []string{}
	for _, cmdV := range cmd {
		for envMapKey, envMapVal := range envMap {
			cmdV = strings.ReplaceAll(cmdV, envMapKey, envMapVal)
		}
		newCmd = append(newCmd, cmdV)
	}
	return newCmd
}
