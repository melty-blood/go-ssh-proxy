package main

import (
	"fmt"
	"honoka/internal/svc"
	"honoka/pkg/confopt"
	"os"
	"slices"
)

func main() {
	var commandArr []string = []string{"sshproxy", "acgpic", "nettouch", "grep"}

	confPath := ""
	for key, val := range os.Args {
		if val == "-f" {
			confPath = os.Args[key+1]
		}
	}
	if confPath == "" {
		confPath = "./conf/conf.yaml"
	}

	var conf *confopt.Config = confopt.ReadConf(confPath)
	commandFlag := ""
	if len(os.Args) <= 2 {
		fmt.Println("len(os.Args) <= 2 ", len(os.Args), os.Args)
		commandFlag = conf.DefaultCommand
		if (len(os.Args) - 1) >= 1 {
			if commandIndex := slices.Index(commandArr, os.Args[1]); commandIndex >= 0 {
				commandFlag = os.Args[1]
			}
		}
	} else {
		commandFlag = os.Args[1]
	}

	svc.CommandRoute(commandFlag)
}
