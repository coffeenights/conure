package applications

import (
	"github.com/gin-gonic/gin"
)

func GenerateRoutes(relativePath string, r *gin.Engine, appHandler *AppHandler) {
	applications := r.Group(relativePath)
	{
		applications.POST("/", appHandler.CreateOrganization)
		applications.GET("/:organizationId", appHandler.GetOrganization)
		applications.POST("/:organizationId/:applicationName/e/", appHandler.CreateEnvironment)
		applications.GET("/:organizationId/:applicationName/e/:environmentId", appHandler.CreateEnvironment)
		applications.GET("/:organizationId/:applicationName/e/:environmentId/:componentName", appHandler.DetailApplications)
		applications.GET("/:organizationId/:applicationName/e/:environmentId/:componentName/:service", appHandler.DetailApplications)
	}
}
