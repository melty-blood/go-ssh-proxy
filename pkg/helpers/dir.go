package helpers

import (
	"os"
	"path/filepath"
)

// 不包含目录
func DirFilesCount(pathDir string) (int, error) {
	fileDirCount := 0
	err := filepath.Walk(pathDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		fileDirCount++
		return nil
	})
	if err != nil {
		return fileDirCount, err
	}
	return fileDirCount, nil
}
