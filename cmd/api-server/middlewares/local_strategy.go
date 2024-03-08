package middlewares

import (
	"github.com/coffeenights/conure/cmd/api-server/auth"
	apiConfig "github.com/coffeenights/conure/cmd/api-server/config"
	"github.com/coffeenights/conure/cmd/api-server/database"
)

type LocalAuthStrategy struct{}

func (l *LocalAuthStrategy) ValidateUser(token string, config *apiConfig.Config, mongo *database.MongoDB) (auth.User,
	error) {
	user := auth.User{}
	claims, err := auth.ValidateToken(token, config.JWTSecret)
	if err != nil {
		return user, auth.ErrUnauthorized
	}

	err = user.GetByEmail(mongo, claims.Data.Email)
	if err != nil {
		return user, auth.ErrUnauthorized
	}
	return user, nil
}
