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
	scanProfileHandler := handler.NewScanProfileHandler(db)

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

		scanProfiles := apiV1.Group("/scan-profiles")
		{
			scanProfiles.POST("", scanProfileHandler.CreateScanProfile)
			scanProfiles.GET("", scanProfileHandler.GetScanProfiles)
			scanProfiles.GET("/:id", scanProfileHandler.GetScanProfileByID)
			scanProfiles.PUT("/:id", scanProfileHandler.UpdateScanProfile)
			scanProfiles.DELETE("/:id", scanProfileHandler.DeleteScanProfile)
		}
	}

	return router
}
