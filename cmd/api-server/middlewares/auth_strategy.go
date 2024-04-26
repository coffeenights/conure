package middlewares

import (
	apiConfig "github.com/coffeenights/conure/cmd/api-server/config"
	"github.com/coffeenights/conure/cmd/api-server/conureerrors"
	"github.com/coffeenights/conure/cmd/api-server/database"
	"github.com/coffeenights/conure/cmd/api-server/models"
)

type AuthStrategy interface {
	ValidateUser(token string, config *apiConfig.Config, mongo *database.MongoDB) (models.User, error)
}

var strategies = map[string]AuthStrategy{
	"local":    &LocalAuthStrategy{},
	"external": &ExternalAuthStrategy{},
}

func ValidateUser(token string, config *apiConfig.Config, mongo *database.MongoDB) (models.User, error) {
	authSystem := config.AuthStrategySystem
	strategy, ok := strategies[authSystem]
	if !ok {
		return models.User{}, conureerrors.ErrWrongAuthenticationSystem
	}
	return strategy.ValidateUser(token, config, mongo)
}
