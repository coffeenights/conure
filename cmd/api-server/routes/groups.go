package routes

import (
	apps "github.com/coffeenights/conure/cmd/api-server/applications"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func GenerateRouter() *gin.Engine {
	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery(), cors.Default())
	app := apps.NewAppHandler()
	apps.GenerateRoutes("/organizations", router, app)
	return router
}
