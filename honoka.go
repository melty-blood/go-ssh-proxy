package main

import (
	"fmt"
	"honoka/internal/svc"
	"honoka/pkg/confopt"
	"log"
	"os"
	"slices"
)

func main() {
	var (
		confPath   = ""
		err        error
		commandArr []string = []string{"sshproxy", "acgpic", "nettouch", "grep", "publish"}
	)
	for key, val := range os.Args {
		if val == "-f" {
			confPath = os.Args[key+1]
		}
	}
	if confPath == "" {
		confPath, err = confopt.GetConfDir()
		if err != nil {
			log.Fatalln("get conf dir fail, err:", err)
		}
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
