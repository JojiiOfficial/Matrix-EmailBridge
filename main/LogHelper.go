package main

import (
	"log"
	"os"
	"strconv"
	"time"
)

var logfile *os.File

const logDir string = "logs/"

func initLogger() {
	ld := dirPrefix + logDir
	if _, err := os.Stat(ld); os.IsNotExist(err) {
		os.Mkdir(ld, 0750)
	}
	var err error
	time := strconv.Itoa(int(time.Now().Unix()))
	logfile, err = os.OpenFile(ld+"access_"+time+".log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		panic(err)
	}
}

type logLevel int

const (
	info     logLevel = 0
	warn     logLevel = 1
	logError logLevel = 2
	critical logLevel = 3
	success  logLevel = 4
)

func loglevelToPrefix(level logLevel) string {
	switch level {
	case info:
		{
			return "[Info]"
		}
	case warn:
		{
			return "[Warning]"
		}
	case logError:
		{
			return "[error]"
		}
	case critical:
		{
			return "[*!critical!*]"
		}
	case success:
		{
			return "[success]"
		}
	}
	return "[low]"
}

func genLogTime() string {
	return "[" + time.Now().Format(time.Stamp) + "]"
}

//WriteLog appends a message to the logfile
func WriteLog(level logLevel, message string) {
	logMessage := genLogTime() + " " + loglevelToPrefix(level) + " " + message + "\r\n"

	if _, err := logfile.Write([]byte(logMessage)); err != nil {
		log.Panic(err)
	}
}
