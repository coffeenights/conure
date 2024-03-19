package applications

import (
	"github.com/gin-gonic/gin"
)

func GenerateRoutes(relativePath string, r *gin.Engine, appHandler *ApiHandler) {
	applications := r.Group(relativePath)
	{
		applications.POST("/", appHandler.CreateOrganization)
		applications.GET("/:organizationID/", appHandler.DetailOrganization)
		applications.GET("/:organizationID/a/", appHandler.ListApplications)
		applications.POST("/:organizationID/a/", appHandler.CreateApplication)
		applications.POST("/:organizationID/a/:applicationID/e/", appHandler.CreateEnvironment)
		applications.DELETE("/:organizationID/a/:applicationID/e/:environment/", appHandler.DeleteEnvironment)
		applications.GET("/:organizationID/a/:applicationID/e/:environment/", appHandler.DetailApplication)
		applications.GET("/:organizationID/a/:applicationID/e/:environment/c/", appHandler.ListComponents)
		applications.GET("/:organizationID/a/:applicationID/e/:environment/c/:componentName/", appHandler.DetailComponent)
		applications.GET("/:organizationID/a/:applicationID/e/:environment/c/:componentName/status/", appHandler.StatusComponent)
	}
}
