package confopt

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/zeromicro/go-zero/core/conf"
)

func ReadConf(filePath string) *Config {
	var c Config
	conf.MustLoad(filePath, &c)

	return &c
}

func ConfKind(conf *Config) map[string]*SSHConfig {
	fmt.Println(conf.ServerConf.SSHConf)
	var confMap map[string]*SSHConfig = make(map[string]*SSHConfig)

	for _, val := range conf.ServerConf.SSHConf {
		fmt.Println(*val)
		confMap[val.ServerName] = val
	}
	return confMap
}

func PrintConfJson(conf *Config) {
	fmt.Println(`------ Print JSON Start ------ `)
	jsonByte, _ := json.Marshal(conf)
	fmt.Println(string(jsonByte))
	fmt.Println(`------ Print JSON End ------ `)
}

func GetConfDir() (string, error) {
	confFileName := "conf.yaml"
	confFilePath := ""
	// check user dir has conf
	confDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	confFilePath = confDir + "/.config/honoka/" + confFileName
	fileInfo, err := os.Stat(confFilePath)
	if err == nil && !fileInfo.IsDir() {
		return confFilePath, nil
	}

	// check execute dir has conf
	confDir, err = os.Executable()
	if err != nil {
		return "", err
	}
	confFilePath = filepath.Dir(confDir) + "/conf/" + confFileName
	fileInfo, err = os.Stat(confFilePath)
	if err == nil && !fileInfo.IsDir() {
		return confFilePath, nil
	}

	return "./conf/conf.yaml", nil
}
