package middlewares

import (
	"github.com/gin-gonic/gin"

	"github.com/coffeenights/conure/cmd/api-server/auth"
	apiConfig "github.com/coffeenights/conure/cmd/api-server/config"
	"github.com/coffeenights/conure/cmd/api-server/conureerrors"
	"github.com/coffeenights/conure/cmd/api-server/database"
)

func CheckAuthenticatedUser(config *apiConfig.Config, mongo *database.MongoDB) gin.HandlerFunc {
	return func(c *gin.Context) {
		authToken, err := c.Cookie("auth")
		if err != nil {
			conureerrors.AbortWithError(c, conureerrors.ErrUnauthorized)
			return
		}
		if authToken == "" {
			conureerrors.AbortWithError(c, conureerrors.ErrUnauthorized)
			return
		}

		_, err = auth.ValidateToken(authToken, config.JWTSecret)
		if err != nil {
			conureerrors.AbortWithError(c, err)
			return
		}

		user, err := ValidateUser(authToken, config, mongo)
		if err != nil {
			conureerrors.AbortWithError(c, err)
			return
		}
		c.Set("currentUser", user)
		c.Next()
	}
}
