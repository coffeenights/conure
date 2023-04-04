package routes

import (
	"github.com/coffeenights/conure/cmd/api-server/applications"
	"github.com/gin-gonic/gin"
)

func GenerateRouter() *gin.Engine {
	router := gin.Default()
	applications.GenerateRoutes("/applications", router)
	return router
}
