package system

import (
	"github.com/gin-gonic/gin"

	apiConfig "github.com/coffeenights/conure/cmd/api-server/config"
	"github.com/coffeenights/conure/cmd/api-server/database"
	"github.com/coffeenights/conure/cmd/api-server/middlewares"
)

// GenerateRoutes wires the system endpoints behind the same auth middleware
// every other authenticated route uses.
func GenerateRoutes(relativePath string, r *gin.Engine, h *Handler, conf *apiConfig.Config, mongo *database.MongoDB) {
	grp := r.Group(relativePath, middlewares.CheckAuthenticatedUser(conf, mongo))
	{
		grp.GET("/info", h.GetInfo)
	}
}
