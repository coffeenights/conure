package middlewares

import (
	"errors"
	"testing"
	"time"

	"github.com/coffeenights/conure/cmd/api-server/auth"
	apiConfig "github.com/coffeenights/conure/cmd/api-server/config"
	"github.com/coffeenights/conure/cmd/api-server/conureerrors"
	"github.com/coffeenights/conure/cmd/api-server/database"
	"github.com/coffeenights/conure/cmd/api-server/models"
)

func TestValidateUserLocal(t *testing.T) {
	payload := auth.JWTData{
		Email:  "test@test.com",
		Client: "test-client",
	}
	token, _ := auth.GenerateToken(1*time.Hour, payload, "test-secret")

	config := &apiConfig.Config{
		MongoDBURI:  "mongodb://localhost:27017",
		MongoDBName: "conure-test",
	}
	mongo, _ := database.ConnectToMongoDB(config.MongoDBURI, config.MongoDBName)
	defer cleanUpDB(mongo)

	user := models.User{
		Email:  "test@test.com",
		Client: "test-client",
	}
	_ = user.Create(mongo)

	// Test cases
	tests := []struct {
		name     string
		token    string
		config   *apiConfig.Config
		expected error
	}{
		{
			name:     "Valid token",
			token:    token,
			config:   &apiConfig.Config{JWTSecret: "test-secret", AuthStrategySystem: "local"},
			expected: nil,
		},
		{
			name:     "Invalid token",
			token:    "test-token",
			config:   &apiConfig.Config{JWTSecret: "test-secret", AuthStrategySystem: "local"},
			expected: conureerrors.ErrUnauthorized,
		},
		{
			name:     "Invalid strategy",
			token:    "test-token",
			config:   &apiConfig.Config{JWTSecret: "test-secret", AuthStrategySystem: "fake-strategy"},
			expected: conureerrors.ErrWrongAuthenticationSystem,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ValidateUser(tt.token, tt.config, mongo)
			if !errors.Is(err, tt.expected) {
				t.Errorf("Expected %v but got %v", tt.expected, err)
			}
		})
	}
}
