package applications

import (
	"github.com/gin-gonic/gin"
)

func GenerateRoutes(relativePath string, r *gin.Engine) {
	applications := r.Group(relativePath)
	{
		applications.GET("/", ListApplications)
		applications.GET("/:applicationName/", DetailApplications)
		applications.GET("/:applicationName/:componentName", DetailApplications)
		applications.GET("/:applicationName/:componentName/:service", DetailApplications)
	}
}
