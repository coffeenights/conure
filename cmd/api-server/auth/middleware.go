package auth

import (
	"net/http"

	"github.com/gin-gonic/gin"

	apiConfig "github.com/coffeenights/conure/cmd/api-server/config"
	"github.com/coffeenights/conure/cmd/api-server/database"
)

func CheckCurrentUser(config *apiConfig.Config, mongo *database.MongoDB) gin.HandlerFunc {
	return func(c *gin.Context) {
		authToken, err := c.Cookie("auth")
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": ErrUnauthorized.Error(),
			})
			return
		}
		if authToken == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": ErrUnauthorized.Error(),
			})
			return
		}

		// validate the token signature and retrieve the jwt payload
		claims, err := ValidateToken(authToken, config.JWTSecret)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": ErrUnauthorized.Error(),
			})
			return
		}

		// get the user data and add it to the context
		user := User{}
		err = user.GetByEmail(mongo, claims.Data.Email)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": ErrUnauthorized.Error(),
			})
			return
		}
		c.Set("currentUser", user)
		c.Next()
	}
}
