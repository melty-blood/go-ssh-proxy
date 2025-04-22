package helpers

import "strings"

func FileExt(fileName string) string {
	indexA := strings.LastIndex(fileName, ".")
	return fileName[indexA:]
}

func FileExtNoPoint(fileName string) string {
	indexA := strings.LastIndex(fileName, ".")
	return fileName[indexA+1:]
}
