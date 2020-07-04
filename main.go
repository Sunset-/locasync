package main

import (
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"os"
	"os/signal"
	"strings"
	"synclocation/logger"
	"synclocation/syncloc"
)

func init() {
	configPath := getAppPath() + "config.yml"
	log.Info("configPath:", configPath)
	viper.SetConfigFile(configPath)
	viper.SetConfigType("yaml")
	if err := viper.ReadInConfig(); err != nil {
		log.Fatal("Fail to read config file :", err)
	}
}

func main() {

	logger.Init()
	//启动同步
	syncloc.Start()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, os.Kill)
	select {
	case <-c:
		break
	}
}

func getAppPath() string {
	return os.Args[0][:(strings.LastIndex(os.Args[0], string(os.PathSeparator)) + 1)]
}
