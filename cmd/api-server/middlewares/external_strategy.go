package middlewares

import (
	"encoding/json"
	"io"
	"net/http"

	apiConfig "github.com/coffeenights/conure/cmd/api-server/config"
	"github.com/coffeenights/conure/cmd/api-server/conureerrors"
	"github.com/coffeenights/conure/cmd/api-server/database"
	"github.com/coffeenights/conure/cmd/api-server/models"
)

type ExternalAuthStrategy struct{}

func (e *ExternalAuthStrategy) ValidateUser(token string, config *apiConfig.Config, _ *database.MongoDB) (models.User,
	error) {
	user := models.User{}
	req, err := http.NewRequest("GET", config.AuthServiceURL, nil)
	if err != nil {
		return user, conureerrors.ErrUnauthorized
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return user, conureerrors.ErrUnauthorized
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return user, conureerrors.ErrUnauthorized
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return user, conureerrors.ErrUnauthorized
	}
	err = json.Unmarshal(body, &user)
	if err != nil {
		return user, conureerrors.ErrUnauthorized
	}

	return user, nil
}
