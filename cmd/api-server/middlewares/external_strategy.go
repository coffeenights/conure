package middlewares

import (
	"encoding/json"
	"github.com/coffeenights/conure/cmd/api-server/models"
	"io"
	"net/http"

	"github.com/coffeenights/conure/cmd/api-server/auth"
	apiConfig "github.com/coffeenights/conure/cmd/api-server/config"
	"github.com/coffeenights/conure/cmd/api-server/database"
)

type ExternalAuthStrategy struct{}

func (e *ExternalAuthStrategy) ValidateUser(token string, config *apiConfig.Config, _ *database.MongoDB) (models.User,
	error) {
	user := models.User{}
	req, err := http.NewRequest("GET", config.AuthServiceURL, nil)
	if err != nil {
		return user, auth.ErrUnauthorized
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return user, auth.ErrUnauthorized
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return user, auth.ErrUnauthorized
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return user, auth.ErrUnauthorized
	}
	err = json.Unmarshal(body, &user)
	if err != nil {
		return user, auth.ErrUnauthorized
	}

	return user, nil
}
