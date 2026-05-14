package users

import (
	"github.com/gin-gonic/gin"

	"github.com/coffeenights/conure/cmd/api-server/middlewares"
)

func GenerateRoutes(relativePath string, r *gin.Engine, handler *Handler) {
	g := r.Group(relativePath,
		middlewares.CheckAuthenticatedUser(handler.Config, handler.MongoDB),
		middlewares.RequireAdmin(),
	)
	{
		g.GET("/", handler.List)
		g.POST("/", handler.Create)
		g.GET("/:userID", handler.Get)
		g.PATCH("/:userID", handler.Update)
		g.DELETE("/:userID", handler.Delete)
		g.POST("/:userID/reset-password", handler.ResetPassword)
	}
}
