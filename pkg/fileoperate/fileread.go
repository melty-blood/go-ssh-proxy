package fileoperate

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func ReadFileOneFunc(filePath string, diyFunc func(lineCon string)) error {
	// 打开文件
	file, err := os.Open(filePath)
	if err != nil {
		fmt.Println("open file error:", err)
		return err
	}
	defer file.Close()

	// 8MB
	scanner := bufio.NewReaderSize(file, 8388608)

	for {
		line, _, err := scanner.ReadLine()
		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Println("scanner.ReadLine err: ", err)
			break
		}
		diyFunc(string(line))
	}
	return nil
}

func ReadFileMaxFunc(filePath string, difFunc func(lineCon string)) error {
	file, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	fileReader := bytes.NewReader(file)
	scanner := bufio.NewScanner(fileReader)

	for scanner.Scan() {
		difFunc(scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return nil
}

// read all dir
func ReadDirFileRecursive(dirPath string, isNeedDirPath bool) ([]string, error) {
	fileDir, err := os.ReadDir(dirPath)
	var pathArr, deepDirArr []string
	var deepDirPath string
	if err != nil {
		return pathArr, err
	}

	for _, fileRes := range fileDir {
		deepDirPath = strings.TrimRight(dirPath, "/") + "/" + fileRes.Name()
		if fileRes.IsDir() {
			deepDirArr, err = ReadDirFileRecursive(deepDirPath, isNeedDirPath)
			if err != nil {
				return pathArr, err
			}
			if !isNeedDirPath {
				deepDirPath = ""
			}
		}
		if len(deepDirPath) > 0 {
			pathArr = append(pathArr, deepDirPath)
		}
		if len(deepDirArr) > 0 {
			pathArr = append(pathArr, deepDirArr...)
		}
		deepDirArr = []string{}
	}
	return pathArr, nil
}

// get only dir
func ReadDirGlob(dirPath string) ([]string, error) {
	var pathArr []string
	pathArr, err := filepath.Glob(dirPath)
	if err != nil {
		return pathArr, nil
	}

	return pathArr, nil
}
