package environments

import (
	"github.com/gin-gonic/gin"
)

func GenerateRoutes(relativePath string, r *gin.Engine) {
	applications := r.Group(relativePath)
	{
		applications.GET("/", ListEnvironments)
	}
}
