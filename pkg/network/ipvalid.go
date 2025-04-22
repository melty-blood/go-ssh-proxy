package network

import (
	"flag"
	"fmt"
	"honoka/pkg/helpers"
	"math/rand"
	"net"
	"os"
	"time"
)

func NetCanTouch(opt *NetTouchOpt) {
	// ip := flag.String("ip", "", "target ip")
	// port := flag.String("port", "", "target port")
	// timeOut := flag.Int("timeout", 6, "timeout")
	// showVersion := flag.Bool("version", false, "Display the version")
	// showV := flag.Bool("V", false, "Shorthand for --version")

	// 解析命令行参数
	// flag.CommandLine.Parse(os.Args[2:])
	// fmt.Println("os.Args[2:]", os.Args[2:])

	// 检查是否显示了版本信息
	if opt.ShowVersion || opt.ShowV {
		fmt.Println("Power by Type-Moon LoveLive IDOLM@STER! -- 6.66")
		os.Exit(0) // 显示版本后退出程序
	}

	if len(opt.Ip) <= 0 || len(opt.Port) <= 0 {
		fmt.Println("please input ip and port, tip: --ip 127.0.0.1 --port 80")
		os.Exit(0)
	}

	if flag.NArg() != 0 {
		r := rand.Intn(3)
		fmt.Println(helpers.GetFailPic(r))
		fmt.Println(r)
		flag.PrintDefaults()
		os.Exit(66)
	}

	TryIpPort(opt.Ip, opt.Port, time.Duration(opt.Timeout))
	fmt.Println("--------------------------------")

}

func TryIpPort(ipStr, port string, timeNum time.Duration) {
	timeout := timeNum * time.Second

	address := net.JoinHostPort(ipStr, port)
	conn, err := net.DialTimeout("tcp", address, timeout)
	if err != nil {
		fmt.Printf("can not connect: %s: %v\n", address, err)
		return
	}
	defer conn.Close()

	fmt.Printf("connect success: %s\n", address)
}

type NetTouchOpt struct {
	Ip, Port           string
	Timeout            int
	ShowVersion, ShowV bool
}
