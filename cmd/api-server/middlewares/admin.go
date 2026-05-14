package middlewares

import (
	"github.com/gin-gonic/gin"

	"github.com/coffeenights/conure/cmd/api-server/conureerrors"
	"github.com/coffeenights/conure/cmd/api-server/models"
)

// RequireAdmin gates a route behind the admin role. It assumes a prior
// CheckAuthenticatedUser middleware has already populated "currentUser".
func RequireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		user, ok := c.MustGet("currentUser").(models.User)
		if !ok || !user.IsAdmin() {
			conureerrors.AbortWithError(c, conureerrors.ErrNotAllowed)
			return
		}
		c.Next()
	}
}
