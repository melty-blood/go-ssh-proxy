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
	"strings"
)

func CommandRoute(commandFlag string) {
	fmt.Println("Run CMD: ", commandFlag)
	commandMap := map[string]func(){
		"sshproxy": RunSSHProxy,
		"nettouch": RunNetTouch,
		"acgpic":   RunACGPic,
		"grep":     RunGrepPro,
		"publish":  RunPublishGit,
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

func RunGrepPro() {
	fmt.Println("Program RunGrepPro")

	showdir := flag.Bool("showdir", false, "print search dir")
	flag.CommandLine.Parse(os.Args[2:])

	if len(os.Args) < 4 {
		fmt.Println("lost search param or search dir!")
		return
	}
	searchStr := os.Args[2]
	searchDir := os.Args[3]
	if *showdir {
		searchStr = os.Args[3]
		searchDir = os.Args[4]
	}
	// fmt.Println("os.Args: ", len(os.Args), searchStr, searchDir, *showdir, " | ", os.Args)
	GrepPro(searchStr, searchDir, *showdir)
}

func RunPublishGit() {
	fmt.Println("Program RunPublishGit")

	fastOrderStr := flag.String("fast-order", "", "fast select git and env")
	confFlag := flag.String("f", "./conf/conf.yaml", "RunACGPic: configure file, default file path ./conf/config.yaml")
	flag.CommandLine.Parse(os.Args[2:])
	conf := confopt.ReadConf(*confFlag)

	var (
		err         error
		gitInfoConf *confopt.PublishGitOpt
	)
	if len(*fastOrderStr) > 0 {
		fastOrderArr := strings.Split(*fastOrderStr, ",")
		if len(fastOrderArr) < 2 {
			fmt.Println("Error fast order failed: must 'KeyName,envNum'")
			return
		}
		fastOrder := &PublishFastOrder{
			GitKey: fastOrderArr[0],
			GitEnv: fastOrderArr[1],
		}
		gitInfoConf, err = PublishFastOrderGit(fastOrder, conf)
	} else {
		gitInfoConf, err = PublishInteractionGit(conf)
	}
	if err != nil {
		fmt.Println("publish err: ", err)
		return
	}

	err = PublishSSH(gitInfoConf)
	if err != nil {
		fmt.Println("PublishSSH err: ", err)
	}
}
