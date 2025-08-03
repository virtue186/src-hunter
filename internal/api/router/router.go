package router

import (
	"github.com/gin-gonic/gin"
	"github.com/hibiken/asynq"
	"github.com/src-hunter/internal/api/handler"
	"github.com/src-hunter/internal/api/middleware"
	"gorm.io/gorm"
)

func SetupRouter(db *gorm.DB, asynqClient *asynq.Client) *gin.Engine {
	router := gin.New()
	router.Use(middleware.LoggerMiddleware())
	router.Use(gin.Recovery())

	projectHandler := handler.NewProjectHandler(db)
	scanHandler := handler.NewScanHandler(db, asynqClient)

	apiV1 := router.Group("/api/v1")
	{
		projects := apiV1.Group("/projects")
		{
			projects.POST("", projectHandler.CreateProject)
			projects.POST("/:projectId/targets", projectHandler.AddTargetsToProject)
		}
		scans := apiV1.Group("/scans")
		{
			scans.POST("", scanHandler.CreateScan)
		}
	}

	return router
}
