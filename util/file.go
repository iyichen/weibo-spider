package util

import (
	"log"
	"os"
	"strings"
)

func Read(filepath string) string {

	bs, err := os.ReadFile(filepath)
	if err != nil {
		log.Printf("Read file error. path: `%s`. e: `%s`. \n", filepath, err)
		return ""
	}
	return strings.TrimSpace(string(bs))
}

func Write(filepath string, content string) {
	_ = os.WriteFile(filepath, []byte(content), 0777)
}

func MakeDir(dir string) bool {
	err := os.MkdirAll(dir, os.ModePerm)
	if err != nil {
		log.Printf("Make dir error. path: `%s`. e: `%s`. \n", dir, err)
		return false
	}
	return true
}
