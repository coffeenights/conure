package applications

import (
	apiConfig "github.com/coffeenights/conure/cmd/api-server/config"
	"github.com/coffeenights/conure/cmd/api-server/database"
)

type ApiHandler struct {
	MongoDB *database.MongoDB
	Config  *apiConfig.Config
}

func NewApiHandler(config *apiConfig.Config, mongo *database.MongoDB) *ApiHandler {
	return &ApiHandler{
		MongoDB: mongo,
		Config:  config,
	}
}
