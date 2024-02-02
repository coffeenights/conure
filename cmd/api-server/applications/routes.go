package applications

import (
	"github.com/gin-gonic/gin"
)

func GenerateRoutes(relativePath string, r *gin.Engine, appHandler *AppHandler) {
	applications := r.Group(relativePath)
	{
		applications.POST("/", appHandler.CreateOrganization)
		applications.GET("/:organizationID", appHandler.GetOrganization)
		applications.POST("/:organizationID/:applicationID/e/", appHandler.CreateEnvironment)
		applications.GET("/:organizationID/:applicationID/e/", appHandler.ListEnvironments)

		applications.GET("/:organizationID/:applicationID/c/", appHandler.CreateEnvironment)
		applications.GET("/:organizationID/:applicationID/c/:componentName", appHandler.CreateEnvironment)
		applications.GET("/:organizationID/:applicationID/e/:environmentID/:componentName", appHandler.DetailApplications)
	}
}
