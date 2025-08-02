package main

import (
	"fmt"
	"github.com/src-hunter/internal/database"
	"github.com/src-hunter/pkg/config"
	"github.com/src-hunter/pkg/logger"
)

func main() {
	// 加载配置
	cfg, err := config.LoadConfig()
	if err != nil {
		fmt.Println(err)
	}
	// 加载日志系统
	err = logger.InitLogger(&cfg.Logger)
	if err != nil {
		fmt.Println(err)
	}
	// 初始化数据库
	db, err := database.InitDB(&cfg.Database)
	if err != nil {
		fmt.Println(err)
	}
	println(db)

}
