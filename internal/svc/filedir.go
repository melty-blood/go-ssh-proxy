package svc

import (
	"fmt"
	"honoka/pkg/fileoperate"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

func GrepPro(searchStr, searchDir string, isShowDir bool) {
	runtime.GOMAXPROCS(runtime.NumCPU() * 2)

	var (
		fileDir  []string
		err      error
		fileStat os.FileInfo
	)
	hasRex := strings.IndexAny(searchDir, "*?[]{}")
	if hasRex >= 0 {
		fileDir, err = filepath.Glob(searchDir)
	} else {
		fileStat, err = os.Stat(searchDir)
		if err != nil {
			fmt.Println("os.Stat: ", err)
			return
		}
		if fileStat.IsDir() {
			fileDir, err = fileoperate.ReadDirFileRecursive(searchDir, false)
		} else {
			fileDir = append(fileDir, searchDir)
		}
	}
	if err != nil {
		fmt.Println("search dir has error: ", err)
		return
	}

	fileCount := len(fileDir)
	if fileCount <= 0 {
		fmt.Println("file not found with path: ", searchDir)
		return
	}

	if isShowDir {
		fmt.Println("find dir path: ", strings.Join(fileDir, "\n"))
		fmt.Print("\n")
	}

	var wg sync.WaitGroup
	wg.Add(fileCount)
	// fmt.Println("fileDir len: ", fileCount)
	readyStr := fmt.Sprintf("\033[1;39;41m%s\033[0m", searchStr)
	lineNum := 1
	for _, filePath := range fileDir {

		go func(filePath string) {
			fileoperate.ReadFileMaxFunc(filePath, func(lineRes string) {
				if strings.Contains(lineRes, searchStr) {
					newStr := strings.Replace(lineRes, searchStr, readyStr, -1)
					fmt.Println(filePath, ":", lineNum, "=>", newStr)
				}
				lineNum++
			})
			wg.Done()
		}(filePath)
	}
	wg.Wait()
	fmt.Println("file search success")
}
