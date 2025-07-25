package svc

import (
	"flag"
	"fmt"
	"honoka/pkg/acgpic"
	"honoka/pkg/confopt"
	"honoka/pkg/helpers"
	"honoka/pkg/network"
	"honoka/pkg/proxysock"
	"os"
)

func CommandRoute(commandFlag string) {

	commandMap := map[string]func(){
		"sshproxy": RunSSHProxy,
		"nettouch": RunNetTouch,
		"acgpic":   RunACGPic,
	}

	funcVal, ok := commandMap[commandFlag]
	if !ok {
		fmt.Println(helpers.GetFailPic(2))
		return
	}

	funcVal()
}

func RunSSHProxy() {
	fmt.Println("Program RunSSHProxy")
	jsonFlag := flag.Bool("json", false, "print config with json")
	whatFlag := flag.Bool("what", false, "ssh proxy 'ssh -NL' command")
	confFlag := flag.String("f", "./conf/conf.yaml", "RunSSHProxy: configure file, default file path ./conf/config.yaml")

	if len(os.Args) > 2 {
		flag.CommandLine.Parse(os.Args[2:])
	} else {
		flag.CommandLine.Parse(os.Args[1:])
	}
	conf := confopt.ReadConf(*confFlag)

	if *whatFlag {
		fmt.Println("this flag is run `ssh -NL`, args from config.yaml")
		return
	}
	if *jsonFlag {
		confopt.PrintConfJson(conf)
		return
	}

	proxysock.UseSSHFunc(conf)
}

func RunNetTouch() {
	fmt.Println("Program RunNetTouch")

	ip := flag.String("ip", "", "target ip")
	port := flag.String("port", "", "target port")
	timeOut := flag.Int("timeout", 6, "timeout")
	showVersion := flag.Bool("version", false, "Display the version")
	showV := flag.Bool("V", false, "Shorthand for --version")
	jsonFlag := flag.Bool("json", false, "print config with json")
	confFlag := flag.String("f", "./conf/conf.yaml", "RunNetTouch: configure file, default file path ./conf/config.yaml")
	conf := confopt.ReadConf(*confFlag)

	// 解析命令行参数
	flag.CommandLine.Parse(os.Args[2:])
	if *jsonFlag {
		confopt.PrintConfJson(conf)
		return
	}

	netOpt := &network.NetTouchOpt{
		Ip:          *ip,
		Port:        *port,
		Timeout:     *timeOut,
		ShowVersion: *showVersion,
		ShowV:       *showV,
	}
	network.NetCanTouch(netOpt)
}

func RunACGPic() {
	fmt.Println("Program RunACGPic")
	targetImgFlag := flag.String("target-img", "", "need search image")
	searchImgDirFlag := flag.String("search-img-dir", "", "search directory")
	thresholdFlag := flag.Int("threshold", 0, "threshold value, this is the similarity of the pictures")
	jsonFlag := flag.Bool("json", false, "print config with json")
	confFlag := flag.String("f", "./conf/conf.yaml", "RunACGPic: configure file, default file path ./conf/config.yaml")
	conf := confopt.ReadConf(*confFlag)

	// 解析命令行参数
	flag.CommandLine.Parse(os.Args[2:])
	if *jsonFlag {
		confopt.PrintConfJson(conf)
		return
	}
	targetImg := conf.AcgPic.TargetImg
	searchImgDir := conf.AcgPic.SearchImgDir
	threshold := conf.AcgPic.Threshold
	if len(*targetImgFlag) > 0 {
		targetImg = *targetImgFlag
	}
	if len(*searchImgDirFlag) > 0 {
		searchImgDir = *searchImgDirFlag
	}
	if *thresholdFlag != 0 {
		threshold = *thresholdFlag
	}

	fmt.Println("----------------")
	fmt.Println("final parms: ", targetImg, searchImgDir, threshold)
	fmt.Println("----------------")
	acgpic.SearchPic(targetImg, searchImgDir, threshold)
}
