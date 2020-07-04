package logger

import (
	"fmt"
	"github.com/spf13/viper"
	"os"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
)

var _logLevelMap = map[string]log.Level{
	"panic": log.PanicLevel,
	"fatal": log.FatalLevel,
	"error": log.ErrorLevel,
	"warn":  log.WarnLevel,
	"info":  log.InfoLevel,
	"debug": log.DebugLevel,
	"trace": log.TraceLevel,
}

func Init() {
	level := log.WarnLevel
	configLogLevel := viper.GetString("logLevel")
	if configLogLevel != "" {
		l, ok := _logLevelMap[strings.ToLower(configLogLevel)]
		if ok {
			level = l
		}
	}
	fmt.Println("LEVEL_LEVEL:",level)

	log.SetLevel(level)

	var currentLogFile *os.File
	var currentLogFileName string
	var err error

	go func() {
		for {
			logfileName := genLogFileName(time.Now())
			if currentLogFile == nil || currentLogFileName != logfileName {
				currentLogFileName = logfileName
				if currentLogFile != nil {
					currentLogFile.Close()
				}
				currentLogFile, err = os.OpenFile(currentLogFileName, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
				if err != nil {
					log.Error("Fail to create log file :", err)
					return
				}
				log.SetOutput(currentLogFile)
				removeLogFile(genLogFileName(time.Now().Add(time.Duration(-72) * time.Hour)))
			}
			time.Sleep(time.Duration(1) * time.Minute)
		}
	}()

}

func genLogFileName(date time.Time) string {
	return getAppPath() + "synclocation." + date.Format("20060102") + ".log"
}

func removeLogFile(logName string) {
	err := os.Remove(logName)
	if err != nil {
		log.Warn("Fail to remove log file:", err)
	} else {
		log.Info("Success to remove log file:", logName)
	}
}


func getAppPath() string {
	return os.Args[0][:(strings.LastIndex(os.Args[0], string(os.PathSeparator)) + 1)]
}
