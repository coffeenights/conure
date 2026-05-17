package credentials

import (
	"github.com/gin-gonic/gin"

	"github.com/coffeenights/conure/cmd/api-server/middlewares"
)

func GenerateRoutes(relativePath string, r *gin.Engine, h *ApiHandler) {
	g := r.Group(relativePath, middlewares.CheckAuthenticatedUser(h.Config, h.MongoDB))
	{
		g.POST("/:organizationID/c", h.CreateCredential)
		g.GET("/:organizationID/c", h.ListCredentials)
		g.DELETE("/:organizationID/c/:name", h.DeleteCredential)
	}
}
