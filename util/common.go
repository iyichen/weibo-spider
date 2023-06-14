package util

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"
)

func init() {
	// init log
	log.SetFlags(log.Ldate | log.Lmicroseconds)
	logFile, err := os.OpenFile("run.log", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		log.Panic("Open log file error.", err)
	}
	log.SetOutput(logFile)
}

func StopTerminalDisappear() {
	time.Sleep(20 * time.Hour)
}

func Escape(name string) string {
	name = strings.Replace(name, "<", "", -1)
	name = strings.Replace(name, ">", "", -1)
	name = strings.Replace(name, ":", "", -1)
	name = strings.Replace(name, "\"", "", -1)
	name = strings.Replace(name, "/", "", -1)
	name = strings.Replace(name, "\\", "", -1)
	name = strings.Replace(name, "|", "", -1)
	name = strings.Replace(name, "?", "", -1)
	name = strings.Replace(name, "*", "", -1)
	return name
}

func Println(content string) {
	fmt.Printf("%s %s.\n", FormatDateTime(time.Now()), content)
}

func FormatDateTime(time time.Time) string {
	return time.Format("2006-01-02 15:04:05")
}
