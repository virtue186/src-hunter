package main

import (
	"fmt"
	"src-hunter/internal/pkg/config"
	"src-hunter/internal/pkg/logger"
)

func main() {
	_, err := config.LoadConfig()
	if err != nil {
		fmt.Println("Error loading config:", err)
	}
	// 初始化 Logger
	logger.InitLogger()
	log := logger.GetLogger()
	defer log.Sync()
	log.Info("日志系统初始化成功!")

	//

}
