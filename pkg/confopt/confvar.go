package confopt

type (
	Config struct {
		DefaultCommand string

		AcgPic struct {
			TargetImg, SearchImgDir string
			Threshold               int
		}

		ServerConf struct {
			SSHConf []*SSHConfig
			Jump    *CommonJump
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

		SockTOHttp struct {
			ServerName string `json:",optional"`
			SockAddr   string `json:",optional"`
			TOHttp     string `json:",optional"`
			OpenStatus bool   `json:",optional"`
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
)
