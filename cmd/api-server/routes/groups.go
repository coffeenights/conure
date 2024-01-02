package routes

import (
	apps "github.com/coffeenights/conure/cmd/api-server/applications"
	envs "github.com/coffeenights/conure/cmd/api-server/environments"
	"github.com/gin-gonic/gin"
)

func GenerateRouter() *gin.Engine {
	router := gin.Default()
	apps.GenerateRoutes("/applications", router)
	envs.GenerateRoutes("/environments", router)
	return router
}
