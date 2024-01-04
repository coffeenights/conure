package applications

import (
	apiConfig "github.com/coffeenights/conure/cmd/api-server/config"
	"github.com/coffeenights/conure/cmd/api-server/database"
	"github.com/coffeenights/conure/internal/config"
	"log"
)

type AppHandler struct {
	MongoDB *database.MongoDB
	Config  *apiConfig.Config
}

func NewAppHandler() *AppHandler {
	appConfig := config.LoadConfig(apiConfig.Config{})
	mongo, err := database.ConnectToMongoDB(appConfig.MongoDBURI)
	if err != nil {
		log.Fatal(err)
	}
	return &AppHandler{
		MongoDB: mongo,
		Config:  appConfig,
	}
}
