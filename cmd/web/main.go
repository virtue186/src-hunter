package main

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"src-hunter/internal/api/router"
	"src-hunter/internal/pkg/config"
	"src-hunter/internal/pkg/database"
	"src-hunter/internal/pkg/logger"
)

func main() {
	// 加载配置文件
	cfg, err := config.LoadConfig()
	if err != nil {
		fmt.Println("Error loading config:", err)
	}
	// 初始化 Logger
	logger.InitLogger()
	log := logger.GetLogger()
	defer log.Sync()
	log.Info("日志系统初始化成功!")

	// 初始化数据库
	log.Info("初始化数据库连接")
	if _, err := database.InitDB(&cfg.Database); err != nil {
		log.Fatalf("Failed to initialize database connection: %v", err)
	}
	db := database.GetDB()

	// 注册路由并开启web服务
	r := gin.Default()
	router.SetupRoutes(r, db)

	if err := r.Run(":" + cfg.Server.Port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
	log.Infof("Starting web server")

}
