// Package users implements the admin-only user management API. The endpoints
// here let an administrator list, create, update, deactivate, and reset
// passwords for any user in the system. Regular users manage their own
// account via /auth (see the auth package).
package users

import (
	apiConfig "github.com/coffeenights/conure/cmd/api-server/config"
	"github.com/coffeenights/conure/cmd/api-server/database"
)

type Handler struct {
	Config  *apiConfig.Config
	MongoDB *database.MongoDB
}

func NewHandler(config *apiConfig.Config, mongo *database.MongoDB) *Handler {
	return &Handler{Config: config, MongoDB: mongo}
}
