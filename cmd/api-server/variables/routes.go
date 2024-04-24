package variables

import (
	"github.com/coffeenights/conure/cmd/api-server/middlewares"
	"github.com/gin-gonic/gin"
)

func GenerateRoutes(relativePath string, r *gin.Engine, handler *Handler) {
	paths := r.Group(relativePath)
	{
		paths.POST("/:organizationID", middlewares.CheckAuthenticatedUser(handler.Config, handler.MongoDB), handler.CreateVariable)
		paths.DELETE("/:organizationID/:variableID", middlewares.CheckAuthenticatedUser(handler.Config, handler.MongoDB), handler.DeleteVariable)
		paths.GET("/:organizationID", middlewares.CheckAuthenticatedUser(handler.Config, handler.MongoDB), handler.ListOrganizationVariables)
		paths.POST("/:organizationID/:applicationID/e/:environmentID", middlewares.CheckAuthenticatedUser(handler.Config, handler.MongoDB), handler.CreateVariable)
		paths.GET("/:organizationID/:applicationID/e/:environmentID", middlewares.CheckAuthenticatedUser(handler.Config, handler.MongoDB), handler.ListEnvironmentVariables)
		paths.POST("/:organizationID/:applicationID/e/:environmentID/c/:componentID", middlewares.CheckAuthenticatedUser(handler.Config, handler.MongoDB), handler.CreateVariable)
		paths.GET("/:organizationID/:applicationID/e/:environmentID/c/:componentID", middlewares.CheckAuthenticatedUser(handler.Config, handler.MongoDB), handler.ListComponentVariables)
	}
}
