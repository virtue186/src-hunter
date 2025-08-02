package logger

import (
	"github.com/src-hunter/pkg/config"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
	"os"
	"strings"
	"time"
)

var Logger *zap.Logger

func InitLogger(cfg *config.LoggerConfig) error {
	// 解析日志级别
	level := zapcore.InfoLevel
	if err := level.UnmarshalText([]byte(strings.ToLower(cfg.Level))); err != nil {
		level = zapcore.InfoLevel
	}

	writeSyncer := getLogWriter(cfg)
	encoder := getEncoder(cfg.Mode)

	var core zapcore.Core
	if strings.ToLower(cfg.Mode) == "dev" {
		consoleEncoder := zapcore.NewConsoleEncoder(zap.NewDevelopmentEncoderConfig())
		core = zapcore.NewTee(
			zapcore.NewCore(consoleEncoder, zapcore.AddSync(os.Stdout), level),
			zapcore.NewCore(encoder, writeSyncer, level),
		)
	} else {
		// 生产环境只写文件
		core = zapcore.NewCore(encoder, writeSyncer, level)
	}

	Logger = zap.New(core, zap.AddCaller(), zap.AddCallerSkip(1))
	zap.ReplaceGlobals(Logger)
	return nil
}

func getEncoder(mode string) zapcore.Encoder {
	encCfg := zap.NewProductionEncoderConfig()
	if strings.ToLower(mode) == "dev" {
		encCfg = zap.NewDevelopmentEncoderConfig()
	}
	encCfg.EncodeTime = func(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
		enc.AppendString(t.Format("2006-01-02 15:04:05"))
	}
	encCfg.EncodeLevel = zapcore.CapitalLevelEncoder
	encCfg.EncodeCaller = zapcore.ShortCallerEncoder
	return zapcore.NewConsoleEncoder(encCfg)
}

func getLogWriter(cfg *config.LoggerConfig) zapcore.WriteSyncer {
	return zapcore.AddSync(&lumberjack.Logger{
		Filename:   cfg.Path,
		MaxSize:    cfg.MaxSize,    // MB
		MaxBackups: cfg.MaxBackups, // 备份文件数量
		MaxAge:     cfg.MaxAge,     // 天数
		Compress:   cfg.Compress,   // 是否压缩
	})
}
