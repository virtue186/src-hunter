package main

import (
	"github.com/hibiken/asynq"
	"github.com/src-hunter/internal/database"
	"github.com/src-hunter/internal/worker"
	"github.com/src-hunter/pkg/config"
	"github.com/src-hunter/pkg/logger"
	"go.uber.org/zap"
	"log"
)

func main() {
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("无法加载配置: %v", err)
	}
	if err := logger.InitLogger(&cfg.Logger, "worker", "log/worker.log"); err != nil {
		log.Fatalf("无法初始化日志: %v", err)
	}
	logger.Logger.Info("日志系统初始化成功")

	db, err := database.InitDB(&cfg.Database)
	if err != nil {
		logger.Logger.Fatal("数据库初始化失败", zap.Error(err))
	}
	logger.Logger.Info("Worker数据库连接成功")

	srv := asynq.NewServer(
		asynq.RedisClientOpt{Addr: cfg.Redis.Addr}, // 从配置中读取Redis地址
		asynq.Config{
			// 指定并发处理任务的数量
			Concurrency: 10,
			// 定义不同优先级的队列
			Queues: map[string]int{
				"critical": 6,
				"default":  3,
				"low":      1,
			},
		},
	)

	mux := asynq.NewServeMux()
	taskProcessor := worker.NewTaskProcessor(db, asynq.NewClient(asynq.RedisClientOpt{Addr: cfg.Redis.Addr}))

	mux.HandleFunc("discovery:subdomain:subfinder", taskProcessor.HandleWorkflowTask)

	logger.Logger.Info("Worker已启动，正在等待任务...")
	if err := srv.Run(mux); err != nil {
		logger.Logger.Fatal("无法启动Worker服务器", zap.Error(err))
	}
}
