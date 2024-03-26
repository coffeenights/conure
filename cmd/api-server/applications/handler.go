package applications

import (
	apiConfig "github.com/coffeenights/conure/cmd/api-server/config"
	"github.com/coffeenights/conure/cmd/api-server/database"
	"github.com/coffeenights/conure/internal/config"
	"log"
)

type ApiHandler struct {
	MongoDB *database.MongoDB
	Config  *apiConfig.Config
}

func NewApiHandler() *ApiHandler {
	appConfig := config.LoadConfig(apiConfig.Config{})
	mongo, err := database.ConnectToMongoDB(appConfig.MongoDBURI, appConfig.MongoDBName)
	if err != nil {
		log.Fatal(err)
	}
	return &ApiHandler{
		MongoDB: mongo,
		Config:  appConfig,
	}
}
