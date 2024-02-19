package applications

import (
	apiConfig "github.com/coffeenights/conure/cmd/api-server/config"
	"github.com/coffeenights/conure/cmd/api-server/database"
	"github.com/coffeenights/conure/internal/config"
	"github.com/gin-gonic/gin"
	_ "github.com/joho/godotenv/autoload"
)

func setupDB() (*database.MongoDB, error) {
	appConfig := config.LoadConfig(apiConfig.Config{})
	testDBName := appConfig.MongoDBName + "-test"
	client, err := database.ConnectToMongoDB(appConfig.MongoDBURI, testDBName)
	if err != nil {
		return nil, err
	}
	return &database.MongoDB{Client: client.Client, DBName: testDBName}, nil
}

func setupRouter() (*gin.Engine, *AppHandler) {
	router := gin.Default()
	app := NewAppHandler()
	app.MongoDB.DBName += "-test"
	GenerateRoutes("/organizations", router, app)
	return router, app
}
