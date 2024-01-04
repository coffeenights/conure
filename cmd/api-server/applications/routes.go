package applications

import (
	"github.com/gin-gonic/gin"
)

func GenerateRoutes(relativePath string, r *gin.Engine, appHandler *AppHandler) {
	applications := r.Group(relativePath)
	{
		applications.GET("/", appHandler.ListApplications)
		applications.GET("/:applicationName/", appHandler.DetailApplications)
		applications.GET("/:applicationName/:componentName", appHandler.DetailApplications)
		applications.GET("/:applicationName/:componentName/:service", appHandler.DetailApplications)
	}
}
