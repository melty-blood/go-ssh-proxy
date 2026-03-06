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
			ServerPriPass  string `json:",optional"`
			JumpHost       string `json:",optional"`
			JumpUser       string `json:",optional"`
			JumpPassword   string `json:",optional"`
			JumpPriKey     string `json:",optional"`
			JumpPriPass    string `json:",optional"`
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
		ServerPriPass  string `json:",optional"`
		JumpHost       string `json:",optional"`
		JumpUser       string `json:",optional"`
		JumpPassword   string `json:",optional"`
		JumpPriKey     string `json:",optional"`
		JumpPriPass    string `json:",optional"`
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
		JumpPriPass  string `json:",optional"`
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
		SSHHost              string                 `json:",optional"`
		SSHPort              string                 `json:",optional"`
		SSHUser              string                 `json:",optional"`
		SSHPasswd            string                 `json:",optional"`
		SSHIdentityFile      string                 `json:",optional"`
		SSHCmd               []string               `json:",optional"`
		IsShowSSHCmdOut      bool                   `json:",optional,default=true"`
		IsSSHCluster         bool                   `json:",optional"`
		SSHCluster           []PublishSSHClusterOpt `json:",optional"`
	}

	PublishSSHClusterOpt struct {
		SSHHost         string
		SSHPort         string
		SSHUser         string
		SSHPasswd       string   `json:",optional"`
		SSHIdentityFile string   `json:",optional"`
		IsUseParentCmd  bool     `json:",optional"`
		SSHCmd          []string `json:",optional"`
		IsShowSSHCmdOut bool     `json:",optional,default=true"`
	}

	PublishGitEnvList struct {
		EnvNum, EnvPath string
	}
)
