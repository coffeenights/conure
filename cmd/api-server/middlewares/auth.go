package middlewares

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/coffeenights/conure/cmd/api-server/auth"
	apiConfig "github.com/coffeenights/conure/cmd/api-server/config"
)

func CheckAuthenticatedUser(config *apiConfig.Config) gin.HandlerFunc {
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

		// get the user data and add it to the context, this will use the /me endpoint to get the user data
		// this middleware must assume that the auth service is an external service
		user := auth.User{}
		req, err := http.NewRequest("GET", config.AuthServiceURL, nil)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": auth.ErrUnauthorized.Error(),
			})
			return
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token[1])

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": auth.ErrUnauthorized.Error(),
			})
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": auth.ErrUnauthorized.Error(),
			})
			return
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": auth.ErrUnauthorized.Error(),
			})
			return
		}
		err = json.Unmarshal(body, &user)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": auth.ErrUnauthorized.Error(),
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
