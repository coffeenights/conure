package middlewares

import (
	"net/http"

	"github.com/coffeenights/conure/cmd/api-server/database"
	"github.com/gin-gonic/gin"

	"github.com/coffeenights/conure/cmd/api-server/auth"
	apiConfig "github.com/coffeenights/conure/cmd/api-server/config"
)

func CheckAuthenticatedUser(config *apiConfig.Config, mongo *database.MongoDB) gin.HandlerFunc {
	return func(c *gin.Context) {
		authToken, err := c.Cookie("auth")
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": auth.ErrUnauthorized.Error(),
			})
			return
		}
		if authToken == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": auth.ErrUnauthorized.Error(),
			})
			return
		}

		_, err = auth.ValidateToken(authToken, config.JWTSecret)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": err.Error(),
			})
			return
		}

		user, err := ValidateUser(authToken, config, mongo)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": err.Error(),
			})
			return
		}
		c.Set("currentUser", user)
		c.Next()
	}
}
