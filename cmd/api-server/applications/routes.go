package applications

import (
	c "github.com/coffeenights/conure/cmd/api-server/applications/controllers"
	"github.com/gin-gonic/gin"
)

func GenerateRoutes(relativePath string, r *gin.Engine) {
	applications := r.Group(relativePath)
	{
		applications.GET("/", c.ListApplications)
	}

}
