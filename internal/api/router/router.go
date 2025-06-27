package router

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"net/http"
)

func SetupRoutes(router *gin.Engine, db *gorm.DB) {
	// 健康检查 API
	// 它可以用来验证服务是否启动，以及数据库连接是否正常
	router.GET("/health", func(c *gin.Context) {
		// 检查数据库连接
		sqlDB, err := db.DB()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"status":  "error",
				"message": "failed to get database connection",
			})
			return
		}

		if err = sqlDB.Ping(); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"status":  "error",
				"message": "database ping failed",
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"status":  "ok",
			"message": "service is healthy",
		})
	})
}
