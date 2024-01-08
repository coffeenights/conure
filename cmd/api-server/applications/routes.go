package applications

import (
	"github.com/gin-gonic/gin"
)

func GenerateRoutes(relativePath string, r *gin.Engine, appHandler *AppHandler) {
	applications := r.Group(relativePath)
	{
		applications.GET("/:organizationId", appHandler.ListApplications)
		applications.GET("/:organizationId/:applicationName/:environmentId", appHandler.DetailApplications)
		applications.GET("/:organizationId/:applicationName/:environmentId/:componentName", appHandler.DetailApplications)
		applications.GET("/:organizationId/:applicationName/:environmentId/:componentName/:service", appHandler.DetailApplications)
	}
}
