package middlewares

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/coffeenights/conure/cmd/api-server/auth"
	apiConfig "github.com/coffeenights/conure/cmd/api-server/config"
	"github.com/coffeenights/conure/cmd/api-server/database"
)

func TestValidateUserExternal(t *testing.T) {
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

	user := auth.User{
		Email:  "test@test.com",
		Client: "test-client",
	}
	_ = user.Create(mongo)

	// Setup a mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/auth":
			jsonResponse, _ := json.Marshal(user)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(jsonResponse)
		case "/auth-bad-data":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"email": 123`))
		case "/auth-bad":
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"status": "error"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	// Test cases
	tests := []struct {
		name     string
		token    string
		config   *apiConfig.Config
		expected error
	}{
		{
			name:  "Valid token",
			token: token,
			config: &apiConfig.Config{JWTSecret: "test-secret", AuthStrategySystem: "external",
				AuthServiceURL: server.URL + "/auth"},
			expected: nil,
		},
		{
			name:  "Invalid token",
			token: "test-token",
			config: &apiConfig.Config{JWTSecret: "test-secret", AuthStrategySystem: "external",
				AuthServiceURL: server.URL + "/auth-bad"},
			expected: auth.ErrUnauthorized,
		}, {
			name:  "Invalid user data",
			token: token,
			config: &apiConfig.Config{JWTSecret: "test-secret", AuthStrategySystem: "external",
				AuthServiceURL: server.URL + "/auth-bad-data"},
			expected: auth.ErrUnauthorized,
		},
		{
			name:     "Invalid strategy",
			token:    "test-token",
			config:   &apiConfig.Config{JWTSecret: "test-secret", AuthStrategySystem: "fake-strategy", AuthServiceURL: server.URL + "/auth"},
			expected: ErrUnsupportedStrategy,
		}, {
			name:  "External does not exists",
			token: token,
			config: &apiConfig.Config{JWTSecret: "test-secret", AuthStrategySystem: "external",
				AuthServiceURL: server.URL + "/404"},
			expected: auth.ErrUnauthorized,
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
