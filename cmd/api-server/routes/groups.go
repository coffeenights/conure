package routes

import (
	apps "github.com/coffeenights/conure/cmd/api-server/applications"
	"github.com/gin-gonic/gin"
)

func GenerateRouter() *gin.Engine {
	router := gin.Default()
	app := apps.NewAppHandler()
	apps.GenerateRoutes("/organizations", router, app)
	return router
}
