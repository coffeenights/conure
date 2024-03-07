package variables

import (
	"github.com/coffeenights/conure/cmd/api-server/middlewares"
	"github.com/gin-gonic/gin"
)

func GenerateRoutes(relativePath string, r *gin.Engine, handler *Handler) {
	paths := r.Group(relativePath)
	{
		paths.POST("/:organizationID", middlewares.CheckAuthenticatedUser(handler.Config), handler.CreateVariable)
		paths.GET("/:organizationID", middlewares.CheckAuthenticatedUser(handler.Config), handler.ListOrganizationVariables)
		paths.POST("/:organizationID/:applicationID/e/:environmentID", middlewares.CheckAuthenticatedUser(handler.Config), handler.CreateVariable)
		paths.GET("/:organizationID/:applicationID/e/:environmentID", middlewares.CheckAuthenticatedUser(handler.Config), handler.ListEnvironmentVariables)
		paths.POST("/:organizationID/:applicationID/e/:environmentID/c/:componentID", middlewares.CheckAuthenticatedUser(handler.Config), handler.CreateVariable)
		paths.GET("/:organizationID/:applicationID/e/:environmentID/c/:componentID", middlewares.CheckAuthenticatedUser(handler.Config), handler.ListComponentVariables)
	}
}
