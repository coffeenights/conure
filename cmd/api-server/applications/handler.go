package applications

import "github.com/coffeenights/conure/cmd/api-server/database"

type AppHandler struct {
	MongoDB *database.MongoDB
}

func NewAppHandler(mongoDB *database.MongoDB) *AppHandler {
	return &AppHandler{MongoDB: mongoDB}
}
