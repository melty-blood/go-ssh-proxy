package tarzip

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func CreateTargz(srcDir, destFile string, isNeedTopDir bool) error {
	// 1. 创建目标文件
	outFile, err := os.Create(destFile)
	if err != nil {
		return errors.New("create targz file failed: " + err.Error())
	}
	defer outFile.Close()

	// create tar.gz file
	gw := gzip.NewWriter(outFile)
	defer gw.Close()
	tarWriter := tar.NewWriter(gw)
	defer tarWriter.Close()

	var filePrefix string
	if srcDir[len(srcDir)-1:] != string(os.PathSeparator) {
		filePrefix = srcDir + string(os.PathSeparator)
	}

	// recursive dir
	return filepath.Walk(srcDir, func(filePath string, fileInfo os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		fileInfo.Name()
		var relPath string
		if isNeedTopDir {
			relPath, err = filepath.Rel(filepath.Dir(srcDir), filePath)
			if err != nil {
				return err
			}
		} else {
			if strings.HasPrefix(filePath, filePrefix) {
				relPath = filePath[len(filePrefix):]
			} else {
				// 当 filePath == sourceDir 时，TrimPrefix 会让 rel == ""
				relPath = strings.TrimPrefix(filePath, srcDir)
				relPath = strings.TrimPrefix(relPath, string(os.PathSeparator))
			}
		}
		// fmt.Println("file path:", filePath, " | ", filepath.Dir(srcDir))
		if relPath == "." || relPath == "" {
			return nil
		}
		// fmt.Println("rel path:", relPath, filePath)

		var linkFile string
		if fileInfo.Mode()&os.ModeSymlink != 0 {
			// 如果是符号链接，读取链接目标以放入 header
			if lk, err := os.Readlink(filePath); err == nil {
				linkFile = lk
			}
		}
		// create tar header
		header, err := tar.FileInfoHeader(fileInfo, linkFile)
		if err != nil {
			return err
		}
		header.Name = relPath
		if err := tarWriter.WriteHeader(header); err != nil {
			return err
		}
		if fileInfo.IsDir() {
			return nil
		}

		fileRes, err := os.Open(filePath)
		if err != nil {
			return err
		}
		defer fileRes.Close()
		// 8MB
		fileReader := bufio.NewReaderSize(fileRes, 8388608)

		buf := make([]byte, 8388608)
		if _, err := io.CopyBuffer(tarWriter, fileReader, buf); err != nil {
			return err
		}
		return nil
	})
}
