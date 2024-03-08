package middlewares

import (
	"github.com/coffeenights/conure/cmd/api-server/auth"
	apiConfig "github.com/coffeenights/conure/cmd/api-server/config"
	"github.com/coffeenights/conure/cmd/api-server/database"
)

type AuthStrategy interface {
	ValidateUser(token string, config *apiConfig.Config, mongo *database.MongoDB) (auth.User, error)
}

var strategies = map[string]AuthStrategy{
	"local":    &LocalAuthStrategy{},
	"external": &ExternalAuthStrategy{},
}

func ValidateUser(token string, config *apiConfig.Config, mongo *database.MongoDB) (auth.User, error) {
	authSystem := config.AuthStrategySystem
	strategy, ok := strategies[authSystem]
	if !ok {
		return auth.User{}, ErrUnsupportedStrategy
	}
	return strategy.ValidateUser(token, config, mongo)
}
