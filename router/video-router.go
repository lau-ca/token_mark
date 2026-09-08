package router

import (
	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/middleware"

	"github.com/gin-gonic/gin"
)

func SetVideoRouter(router *gin.Engine) {
	seedanceV3Router := router.Group("/v3")
	seedanceV3Router.Use(middleware.RouteTag("relay"))
	seedanceV3Router.Use(middleware.TokenAuth())
	{
		seedanceV3Router.POST("/contents/generations/tasks", middleware.Distribute(), controller.RelayTask)
		seedanceV3Router.GET("/contents/generations/tasks", controller.SeedanceTaskList)
		seedanceV3Router.GET("/contents/generations/tasks/:task_id", controller.SeedanceTaskFetch)
		seedanceV3Router.DELETE("/contents/generations/tasks/:task_id", controller.SeedanceTaskDelete)
	}

	qianfanVideoRouter := router.Group("/qianfan/v1")
	qianfanVideoRouter.Use(middleware.RouteTag("relay"))
	qianfanVideoRouter.Use(middleware.TokenAuth(), middleware.Distribute())
	{
		qianfanVideoRouter.POST("/videos", controller.RelayTask)
		qianfanVideoRouter.GET("/videos/:task_id", controller.RelayTaskFetch)
	}

	seedanceAssetRouter := router.Group("/volc")
	seedanceAssetRouter.Use(middleware.RouteTag("relay"))
	seedanceAssetRouter.Use(middleware.TokenAuth())
	{
		seedanceAssetRouter.POST("/ark", controller.SeedanceAssetProxy)
	}

	seedanceRegisteredAssetRouter := router.Group("/v1/volc")
	seedanceRegisteredAssetRouter.Use(middleware.RouteTag("relay"))
	seedanceRegisteredAssetRouter.Use(middleware.TokenAuth(), middleware.Distribute())
	{
		seedanceRegisteredAssetRouter.POST("/ark", controller.SeedanceAssetProxy)
	}

	videoV1Router := router.Group("/v1")
	videoV1Router.Use(middleware.RouteTag("relay"))
	videoV1Router.Use(middleware.TokenAuth(), middleware.Distribute())
	{
		videoV1Router.POST("/video/generations", controller.RelayTask)
		videoV1Router.GET("/video/generations/:task_id", controller.RelayTaskFetch)
		videoV1Router.POST("/videos/:video_id/remix", controller.RelayTask)
	}
	klingV1Router := router.Group("/kling/v1")
	klingV1Router.Use(middleware.RouteTag("relay"))
	klingV1Router.Use(middleware.KlingRequestConvert(), middleware.TokenAuth(), middleware.Distribute())
	{
		klingV1Router.POST("/videos/text2video", controller.RelayTask)
		klingV1Router.POST("/videos/image2video", controller.RelayTask)
		klingV1Router.GET("/videos/text2video/:task_id", controller.RelayTaskFetch)
		klingV1Router.GET("/videos/image2video/:task_id", controller.RelayTaskFetch)
	}

	// Jimeng official API routes - direct mapping to official API format
	jimengOfficialGroup := router.Group("jimeng")
	jimengOfficialGroup.Use(middleware.RouteTag("relay"))
	jimengOfficialGroup.Use(middleware.JimengRequestConvert(), middleware.TokenAuth(), middleware.Distribute())
	{
		// Maps to: /?Action=CVSync2AsyncSubmitTask&Version=2022-08-31 and /?Action=CVSync2AsyncGetResult&Version=2022-08-31
		jimengOfficialGroup.POST("/", controller.RelayTask)
	}
}
