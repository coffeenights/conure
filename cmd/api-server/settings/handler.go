package settings

import (
	apiConfig "github.com/coffeenights/conure/cmd/api-server/config"
	"github.com/coffeenights/conure/cmd/api-server/database"
	"github.com/coffeenights/conure/cmd/api-server/variables"
)

type ApiHandler struct {
	MongoDB    *database.MongoDB
	Config     *apiConfig.Config
	keyStorage variables.SecretKeyStorage
}

func NewApiHandler(config *apiConfig.Config, mongo *database.MongoDB,
	keyStorage variables.SecretKeyStorage) *ApiHandler {
	return &ApiHandler{
		MongoDB:    mongo,
		Config:     config,
		keyStorage: keyStorage,
	}
}
