package applications

import (
	apiConfig "github.com/coffeenights/conure/cmd/api-server/config"
	"github.com/coffeenights/conure/cmd/api-server/models"
	"github.com/coffeenights/conure/internal/config"
	"github.com/gin-gonic/gin"
	_ "github.com/joho/godotenv/autoload"
	"log"
)

func setupRouter() (*gin.Engine, *ApiHandler) {
	router := gin.Default()
	db, err := models.SetupDB()
	appConfig := config.LoadConfig(apiConfig.Config{})
	appConfig.MongoDBName = appConfig.MongoDBName + "-test"
	if err != nil {
		log.Panic(err)
	}
	app := NewApiHandler(appConfig, db)
	GenerateRoutes("/organizations", router, app)
	return router, app
}
