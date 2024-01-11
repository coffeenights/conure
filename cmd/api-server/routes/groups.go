package routes

import (
	"github.com/gin-gonic/gin"

	apps "github.com/coffeenights/conure/cmd/api-server/applications"
	"github.com/coffeenights/conure/cmd/api-server/auth"
	apiConfig "github.com/coffeenights/conure/cmd/api-server/config"
	"github.com/coffeenights/conure/cmd/api-server/database"
	"github.com/coffeenights/conure/internal/config"
)

func GenerateRouter() *gin.Engine {
	router := gin.Default()
	conf := config.LoadConfig(apiConfig.Config{})
	mongo, err := database.ConnectToMongoDB(conf.MongoDBURI, conf.MongoDBName)
	if err != nil {
		panic(err)
	}

	appHandler := apps.NewAppHandler()
	authHandler := auth.NewAuthHandler(conf, mongo)
	apps.GenerateRoutes("/organizations", router, appHandler)
	auth.GenerateRoutes("/auth", router, authHandler)
	return router
}
