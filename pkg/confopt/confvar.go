package confopt

type (
	Config struct {
		DefaultCommand string

		AcgPic struct {
			TargetImg, SearchImgDir string
			Threshold               int
		}

		ServerConf struct {
			SignalOrderFilePath string
			SSHConf             []*SSHConfig
			Jump                *CommonJump
		}

		SockProxy struct {
			OpenStatus     bool
			NeedJump       bool
			IsError        bool `json:",optional"`
			ServerName     string
			ServerHost     string
			ServerUser     string
			ServerPassword string
			ServerPriKey   string
			JumpHost       string `json:",optional"`
			JumpUser       string `json:",optional"`
			JumpPassword   string `json:",optional"`
			JumpPriKey     string `json:",optional"`
			Local          string
			Proxy          string
			SSHCommand     string `json:",optional"`
		}

		SockToHttp struct {
			ServerName string `json:",optional"`
			SockAddr   string `json:",optional"`
			ToHttp     string `json:",optional"`
			OpenStatus bool   `json:",optional"`
		}

		Publish struct {
			GitList []*PublishGitOpt
		}
	}
	SSHConfig struct {
		ServerName     string
		ServerHost     string
		ServerUser     string
		ServerPassword string
		ServerPriKey   string
		JumpHost       string `json:",optional"`
		JumpUser       string `json:",optional"`
		JumpPassword   string `json:",optional"`
		JumpPriKey     string `json:",optional"`
		Local          string
		Proxy          string
		OpenStatus     bool
		NeedJump       bool
		IsError        bool `json:",optional"`
	}

	CommonJump struct {
		JumpHost     string `json:",optional"`
		JumpUser     string `json:",optional"`
		JumpPassword string `json:",optional"`
		JumpPriKey   string `json:",optional"`
	}

	PublishGitOpt struct {
		KeyName              string
		RepoUrl              string
		ClonePath            string
		TargzPath            string
		TargzIsNeedTopDir    bool
		CheckBranch          string
		RemoteName           string
		RemoteBranch         string
		SSHGitUser           string `json:",optional"`
		SSHGitIdentityFile   string `json:",optional"`
		SSHGitIdentityPasswd string `json:",optional"`
		HttpsGitUser         string `json:",optional"`
		HttpsGitPat          string `json:",optional"`
		EnvList              []PublishGitEnvList
		SelectEnv            string `json:",optional"`
		SftpUploadPath       string
		SSHHost              string
		SSHPort              string
		SSHUser              string
		SSHPasswd            string
		SSHCmd               []string
	}

	PublishGitEnvList struct {
		EnvNum, EnvPath string
	}
)
