package auth

import (
	"github.com/gin-gonic/gin"

	apiConfig "github.com/coffeenights/conure/cmd/api-server/config"
	"github.com/coffeenights/conure/cmd/api-server/conureerrors"
	"github.com/coffeenights/conure/cmd/api-server/database"
	"github.com/coffeenights/conure/cmd/api-server/models"
)

func CheckCurrentUser(config *apiConfig.Config, mongo *database.MongoDB) gin.HandlerFunc {
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

		// validate the token signature and retrieve the jwt payload
		claims, err := ValidateToken(authToken, config.JWTSecret)
		if err != nil {
			conureerrors.AbortWithError(c, err)
			return
		}

		// get the user data and add it to the context
		user := models.User{}
		err = user.GetByEmail(mongo, claims.Data.Email)
		if err != nil {
			conureerrors.AbortWithError(c, conureerrors.ErrUnauthorized)
			return
		}
		c.Set("currentUser", user)
		c.Next()
	}
}
