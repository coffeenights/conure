package settings

import (
	"github.com/gin-gonic/gin"

	"github.com/coffeenights/conure/cmd/api-server/middlewares"
)

func GenerateRoutes(relativePath string, r *gin.Engine, appHandler *ApiHandler) {
	applications := r.Group(relativePath, middlewares.CheckAuthenticatedUser(appHandler.Config, appHandler.MongoDB))
	{
		applications.POST("/:organizationID/i", appHandler.CreateIntegration)
		applications.GET("/:organizationID/i", appHandler.ListIntegrations)
		applications.DELETE("/:organizationID/i/:integrationID", appHandler.DeleteIntegration)
	}
}
