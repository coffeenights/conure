package routes

import (
	apps "github.com/coffeenights/conure/cmd/api-server/applications"
	"github.com/coffeenights/conure/cmd/api-server/database"
	"github.com/gin-gonic/gin"
	"log"
)

func GenerateRouter() *gin.Engine {
	router := gin.Default()
	handler, err := database.ConnectToMongoDB()
	if err != nil {
		log.Fatal(err)
	}
	app := apps.NewAppHandler(handler)
	apps.GenerateRoutes("/applications", router, app)
	return router
}
