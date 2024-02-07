package applications

import (
	"github.com/gin-gonic/gin"
)

func GenerateRoutes(relativePath string, r *gin.Engine, appHandler *AppHandler) {
	applications := r.Group(relativePath)
	{
		applications.POST("/", appHandler.CreateOrganization)
		applications.GET("/:organizationID/", appHandler.ListApplications)
		applications.POST("/:organizationID/:applicationID/e/", appHandler.CreateEnvironment)
		applications.GET("/:organizationID/:applicationID/e/", appHandler.ListEnvironments)
		applications.DELETE("/:organizationID/:applicationID/e/:environmentID", appHandler.DeleteEnvironment)

		applications.GET("/:organizationID/:applicationID/c/", appHandler.CreateEnvironment)
		applications.GET("/:organizationID/:applicationID/c/:componentName", appHandler.CreateEnvironment)
		applications.GET("/:organizationID/:applicationID/e/:environmentID/:componentName", appHandler.DetailApplications)
	}
}
