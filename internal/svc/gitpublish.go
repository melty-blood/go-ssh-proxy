package svc

import (
	"bufio"
	"errors"
	"fmt"
	"honoka/pkg/confopt"
	"honoka/pkg/network/sshcmd"
	"honoka/pkg/tarzip"
	"os"
	"slices"
	"strings"
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

func TempSSH(conf *confopt.Config) {
	var err error
	// err = tarzip.CreateTargz("/dev/shm/temp/numchange_fix_data", "/dev/shm/num.tar.gz", false)
	// err = tarzip.CreateTargz("/dev/shm/individual_verify.csv", "/dev/shm/individual_verify_go.tar.gz")
	fmt.Println("CreateTargz err: ", err)

	sshConf := &sshcmd.SSHConf{
		User:         "akiha",
		Password:     "lovelive",
		IdentityFile: "",
		Host:         "192.168.51.198",
		Port:         "28606",
	}
	conn, _ := sshcmd.SSHConnect(sshConf)
	cmdOut, err := sshcmd.RunCmdWithSSH([]string{"pwd", "ls -latr", "cd /dev/shm", "pwd", "ls /nodir", "ls /root/*"}, conn)
	fmt.Println("cmdOut: ", cmdOut, err)
	fmt.Println("----------------------------------------------------")
	for _, v := range cmdOut[1:] {
		fmt.Println("cmdOut+++++++++++: ", v)
	}
	if true {
		os.Exit(66)
	}

	fileMap := []*sshcmd.SftpFile{
		{
			LFilePath: "/dev/shm/temp/not_sign.txt",
			RFilePath: "/home/akiha/lovelive/not_sign.txt",
		},
		{
			LFilePath: "/dev/shm/temp/qinyidai_no_file.txt",
			RFilePath: "/home/akiha/lovelive/qinyidai_no_file.txt",
		},
	}
	err = sshcmd.SSHUploadFile(fileMap, conn)
	fmt.Println("err: ", err)
}

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
	if publishPath[len(publishPath)-1:] != string(os.PathSeparator) {
		publishPath = publishPath + string(os.PathSeparator)
	}
	fmt.Println("publishPath:", publishPath)

	uploadNameArr := strings.Split(gitConf.SftpUploadPath, string(os.PathSeparator))
	packageTargzName := uploadNameArr[len(uploadNameArr)-1]
	fileName := publishPath + packageTargzName
	fmt.Println("fileName:", packageTargzName, fileName)

	// init ENV_MAP
	envArr := []string{
		envName + "|" + gitConf.SelectEnv,
		sshEnvPath + "|" + publishPath,
		uploadPath + "|" + gitConf.SftpUploadPath,
		packageName + "|" + packageTargzName,
		packagePath + "|" + fileName,
	}
	envMap := InitEnvParam(envArr)

	newCmd := []string{}
	for _, cmdV := range gitConf.SSHCmd {
		for envMapKey, envMapVal := range envMap {
			cmdV = strings.ReplaceAll(cmdV, envMapKey, envMapVal)
		}
		newCmd = append(newCmd, cmdV)
	}

	err = tarzip.CreateTargz(gitConf.ClonePath, gitConf.TargzPath, gitConf.TargzIsNeedTopDir)
	if err != nil {
		return err
	}

	sshConf := &sshcmd.SSHConf{
		User:         gitConf.SSHUser,
		Password:     gitConf.SSHPasswd,
		IdentityFile: "",
		Host:         gitConf.SSHHost,
		Port:         gitConf.SSHPort,
	}
	conn, _ := sshcmd.SSHConnect(sshConf)
	fileMap := []*sshcmd.SftpFile{
		{
			LFilePath: gitConf.TargzPath,
			RFilePath: gitConf.SftpUploadPath,
		},
	}
	err = sshcmd.SSHUploadFile(fileMap, conn)
	if err != nil {
		return err
	}
	cmdRes, err := sshcmd.RunCmdWithSSH(newCmd, conn)
	if err != nil {
		return err
	}

	for _, cmdV := range cmdRes {
		fmt.Println(strings.Trim(cmdV, ""))
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
	fmt.Println("fastOrder:", fastOrder.GitKey, "--", fastOrder.GitEnv)

	return PublishCode(confGitInfo)
}

func PublishInteractionGit(conf *confopt.Config) (*confopt.PublishGitOpt, error) {
	var (
		selectGitInput, selectEnvInput string
		envList                        []string
	)
	gitMap, gitkeyArr := buildPublishMap(conf)

	// select env
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
	// git clone
	gitRep, err := git.PlainOpenWithOptions(opt.ClonePath, &git.PlainOpenOptions{})
	if err != nil && errors.Is(err, git.ErrRepositoryNotExists) {
		fmt.Println("ready clone remote:", err)
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
	fmt.Printf("Find remote branche: '%s', point at Commit: %s \n", remoteRefName.Short(), remoteCommitHash)

	// show HEAD
	gitReference, err := gitRep.Head()
	if err != nil {
		return nil, errors.New("git show Head failed:" + err.Error())
	}
	workTree, err := gitRep.Worktree()
	if gitReference.Name().Short() != opt.CheckBranch {

		fmt.Println("gitRep.workTree: ", err)

		err = workTree.Checkout(&git.CheckoutOptions{
			// Hash: 新分支将指向这个 Commit
			Hash:   remoteCommitHash,
			Branch: plumbing.NewBranchReferenceName(opt.CheckBranch),
			Create: true,
		})
		if err != nil {
			return nil, errors.New("git checkout branch failed:" + err.Error())
		}
	}
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
		fmt.Println("log: ", logsV)
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
	fmt.Println("git pull head name:", head.Name().Short())
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
		fmt.Println("git pull err NoErrAlreadyUpToDate:", err)
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
