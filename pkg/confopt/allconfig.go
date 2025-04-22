package confopt

import (
	"encoding/json"
	"fmt"

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
