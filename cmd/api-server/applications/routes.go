package applications

import (
	"github.com/coffeenights/conure/cmd/api-server/middlewares"
	"github.com/gin-gonic/gin"
)

func GenerateRoutes(relativePath string, r *gin.Engine, appHandler *ApiHandler) {
	applications := r.Group(relativePath, middlewares.CheckAuthenticatedUser(appHandler.Config, appHandler.MongoDB))
	{
		applications.GET("/", appHandler.ListOrganization)
		applications.POST("/", appHandler.CreateOrganization)
		applications.GET("/:organizationID", appHandler.DetailOrganization)
		applications.GET("/:organizationID/a", appHandler.ListApplications)
		applications.POST("/:organizationID/a", appHandler.CreateApplication)
		applications.POST("/:organizationID/a/:applicationID/e", appHandler.CreateEnvironment)
		applications.DELETE("/:organizationID/a/:applicationID/e/:environment", appHandler.DeleteEnvironment)
		applications.PUT("/:organizationID/a/:applicationID/e/:environment", appHandler.DeployApplication)
		applications.GET("/:organizationID/a/:applicationID/e/:environment", appHandler.DetailApplication)
		applications.GET("/:organizationID/a/:applicationID/e/:environment/status", appHandler.StatusApplication)
		applications.GET("/:organizationID/a/:applicationID/e/:environment/c", appHandler.ListComponents)
		applications.POST("/:organizationID/a/:applicationID/e/:environment/c", appHandler.CreateComponent)
		applications.GET("/:organizationID/a/:applicationID/e/:environment/c/:componentID", appHandler.DetailComponent)
		applications.GET("/:organizationID/a/:applicationID/e/:environment/c/:componentID/status", appHandler.StatusComponent)
	}
}
