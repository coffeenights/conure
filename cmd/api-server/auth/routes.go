package auth

import (
	"github.com/gin-gonic/gin"
)

func GenerateRoutes(relativePath string, r *gin.Engine, handler *Handler) {
	paths := r.Group(relativePath)
	{
		paths.POST("/login", handler.Login)
		paths.GET("/me", CheckCurrentUser(handler.Config, handler.MongoDB), handler.Me)
		paths.PATCH("/change-password", CheckCurrentUser(handler.Config, handler.MongoDB), handler.ChangePassword)
	}
}
