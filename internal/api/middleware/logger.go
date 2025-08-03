package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/src-hunter/pkg/logger"
	"go.uber.org/zap"
	"time"
)

func LoggerMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 请求开始时间
		start := time.Now()
		// 获取请求的路径和方法
		path := c.Request.URL.Path
		method := c.Request.Method

		// 处理请求
		c.Next()

		// 请求处理结束后的相关信息
		cost := time.Since(start)
		statusCode := c.Writer.Status()
		clientIP := c.ClientIP()
		userAgent := c.Request.UserAgent()
		errors := c.Errors.ByType(gin.ErrorTypePrivate).String()

		// 根据状态码判断日志级别
		var fields []zap.Field
		switch {
		case statusCode >= 400 && statusCode < 500:
			// 4xx 客户端错误，记录为 Warn
			fields = []zap.Field{
				zap.Int("status_code", statusCode),
				zap.String("method", method),
				zap.String("path", path),
				zap.String("client_ip", clientIP),
				zap.String("user_agent", userAgent),
				zap.String("cost", cost.String()),
				zap.String("errors", errors),
			}
			logger.Logger.Warn("HTTP request", fields...)
		case statusCode >= 500:
			// 5xx 服务器错误，记录为 Error
			fields = []zap.Field{
				zap.Int("status_code", statusCode),
				zap.String("method", method),
				zap.String("path", path),
				zap.String("client_ip", clientIP),
				zap.String("user_agent", userAgent),
				zap.String("cost", cost.String()),
				zap.String("errors", errors),
			}
			logger.Logger.Error("HTTP request", fields...)
		default:
			// 其他情况（如 2xx, 3xx），记录为 Info
			fields = []zap.Field{
				zap.Int("status_code", statusCode),
				zap.String("method", method),
				zap.String("path", path),
				zap.String("client_ip", clientIP),
				zap.String("user_agent", userAgent),
				zap.String("cost", cost.String()),
			}
			logger.Logger.Info("HTTP request", fields...)
		}
	}
}
