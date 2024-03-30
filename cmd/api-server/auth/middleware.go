package auth

import (
	"github.com/coffeenights/conure/cmd/api-server/models"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	apiConfig "github.com/coffeenights/conure/cmd/api-server/config"
	"github.com/coffeenights/conure/cmd/api-server/database"
)

func CheckCurrentUser(config *apiConfig.Config, mongo *database.MongoDB) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": ErrUnauthorized.Error(),
			})
			return
		}

		token := strings.Split(authHeader, " ")
		if len(token) != 2 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": ErrUnauthorized.Error(),
			})
			return
		}

		if token[0] != "Bearer" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": ErrUnauthorized.Error(),
			})
			return
		}

		// validate the token signature and retrieve the jwt payload
		claims, err := ValidateToken(token[1], config.JWTSecret)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": ErrUnauthorized.Error(),
			})
			return
		}

		// get the user data and add it to the context
		user := models.User{}
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
