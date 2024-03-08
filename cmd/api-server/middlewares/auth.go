package middlewares

import (
	"net/http"
	"strings"

	"github.com/coffeenights/conure/cmd/api-server/database"
	"github.com/gin-gonic/gin"

	"github.com/coffeenights/conure/cmd/api-server/auth"
	apiConfig "github.com/coffeenights/conure/cmd/api-server/config"
)

func CheckAuthenticatedUser(config *apiConfig.Config, mongo *database.MongoDB) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": auth.ErrUnauthorized.Error(),
			})
			return
		}

		token := strings.Split(authHeader, " ")
		err := validateSignature(token, config)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": err.Error(),
			})
			return
		}

		user, err := ValidateUser(token[1], config, mongo)
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

func validateSignature(token []string, config *apiConfig.Config) error {
	if len(token) != 2 {
		return auth.ErrUnauthorized
	}

	if token[0] != "Bearer" {
		return auth.ErrUnauthorized
	}

	// validate the token signature
	_, err := auth.ValidateToken(token[1], config.JWTSecret)
	if err != nil {
		return auth.ErrUnauthorized
	}
	return nil
}
