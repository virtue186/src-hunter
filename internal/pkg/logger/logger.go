package logger

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"os"
)

var log *zap.SugaredLogger

func InitLogger() {
	// 配置日志编码器
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "time",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.CapitalLevelEncoder, // 大写编码器
		EncodeTime:     zapcore.ISO8601TimeEncoder,  // ISO8601 UTC 时间格式
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder, // 短路径编码器
	}

	// 设置日志级别
	atomicLevel := zap.NewAtomicLevel()
	atomicLevel.SetLevel(zap.InfoLevel) // 设置为 Info 级别，可以从配置文件读取

	// 创建核心
	core := zapcore.NewCore(
		// zapcore.NewConsoleEncoder(encoderConfig), // 控制台格式
		zapcore.NewJSONEncoder(encoderConfig),                   // JSON 格式
		zapcore.NewMultiWriteSyncer(zapcore.AddSync(os.Stdout)), // 同时输出到控制台
		// zapcore.NewMultiWriteSyncer(zapcore.AddSync(os.Stdout), zapcore.AddSync(getLogWriter())), // 同时输出到控制台和文件
		atomicLevel,
	)

	// 构建 Logger
	// zap.AddCaller() 会添加调用函数的文件名和行号
	// zap.Development() 会在 DPanicLevel 的日志中 panic
	logger := zap.New(core, zap.AddCaller(), zap.Development())

	// 设置为全局 logger
	log = logger.Sugar()
}

// GetLogger 提供一个全局访问 logger 的方法
func GetLogger() *zap.SugaredLogger {
	return log
}
