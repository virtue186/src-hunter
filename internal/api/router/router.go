package router

import (
	"github.com/gin-contrib/cors"
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

	config := cors.DefaultConfig()
	// 允许来自 Vite 开发服务器 (默认端口5173) 的请求
	config.AllowOrigins = []string{"http://localhost:5173"}
	config.AllowMethods = []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}
	config.AllowHeaders = []string{"Origin", "Content-Length", "Content-Type", "Authorization"}
	router.Use(cors.New(config))

	projectHandler := handler.NewProjectHandler(db)
	scanHandler := handler.NewScanHandler(db, asynqClient)
	scanProfileHandler := handler.NewScanProfileHandler(db)
	taskHandler := handler.NewTaskHandler(db)
	domainHandler := handler.NewDomainHandler(db)

	apiV1 := router.Group("/api/v1")
	{
		projects := apiV1.Group("/projects")
		{
			projects.GET("", projectHandler.GetProjects)
			projects.GET("/:projectId", projectHandler.GetProjectByID)
			projects.POST("", projectHandler.CreateProject)
			projects.POST("/:projectId/targets", projectHandler.AddTargetsToProject)
			projects.GET("/:projectId/tasks", taskHandler.GetTasksByProject)
			projects.GET("/:projectId/domains", domainHandler.GetDomainsByProject)
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
