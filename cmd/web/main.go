package main

import (
	"fmt"
	"github.com/hibiken/asynq"
	"github.com/src-hunter/internal/api/router"
	"github.com/src-hunter/internal/database"
	"github.com/src-hunter/pkg/config"
	"github.com/src-hunter/pkg/logger"
	"go.uber.org/zap"
)

func main() {
	// 加载配置
	cfg, err := config.LoadConfig()
	if err != nil {
		fmt.Println(err)
	}
	// 加载日志系统
	err = logger.InitLogger(&cfg.Logger, "web", "log/web.log")
	if err != nil {
		fmt.Println(err)
	}
	// 初始化数据库
	db, err := database.InitDB(&cfg.Database)
	if err != nil {
		fmt.Println(err)
	}

	redisOpt := asynq.RedisClientOpt{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	}
	asynqClient := asynq.NewClient(redisOpt)
	defer asynqClient.Close()

	r := router.SetupRouter(db, asynqClient)
	addr := fmt.Sprintf(":%s", cfg.Server.Port)
	logger.Logger.Info("Server is running on ", zap.String("addr", addr))

	err = r.Run(addr)
	if err != nil {
		logger.Logger.Error("Server is shutting down", zap.Error(err))
	}

}
