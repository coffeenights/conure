package middlewares

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/coffeenights/conure/cmd/api-server/auth"
	apiConfig "github.com/coffeenights/conure/cmd/api-server/config"
	"github.com/coffeenights/conure/cmd/api-server/database"
	"github.com/gin-gonic/gin"
)

func cleanUpDB(mongo *database.MongoDB) {
	err := mongo.Client.Database(mongo.DBName).Drop(context.Background())
	if err != nil {
		panic(err)
	}
}

func TestValidateSignature(t *testing.T) {
	// Test cases
	payload := auth.JWTData{
		Email:  "test@test.com",
		Client: "test",
	}
	token, _ := auth.GenerateToken(1*time.Hour, payload, "test-secret")
	tests := []struct {
		name     string
		token    []string
		config   *apiConfig.Config
		expected error
	}{
		{
			name:     "Valid token",
			token:    []string{"Bearer", token},
			config:   &apiConfig.Config{JWTSecret: "test-secret"},
			expected: nil,
		},
		{
			name:     "Invalid token",
			token:    []string{"Bearer", "test-token"},
			config:   &apiConfig.Config{JWTSecret: "invalid-secret"},
			expected: auth.ErrUnauthorized,
		},
		{
			name:     "Invalid token format",
			token:    []string{"test-token"},
			config:   &apiConfig.Config{JWTSecret: "test-secret"},
			expected: auth.ErrUnauthorized,
		},
		{
			name:     "Invalid token type",
			token:    []string{"Invalid", "test-token"},
			config:   &apiConfig.Config{JWTSecret: "test-secret"},
			expected: auth.ErrUnauthorized,
		},
		{
			name:     "Empty token",
			token:    []string{},
			config:   &apiConfig.Config{JWTSecret: "test-secret"},
			expected: auth.ErrUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSignature(tt.token, tt.config)
			if !errors.Is(err, tt.expected) {
				t.Errorf("Expected %v, got %v", tt.expected, err)
			}
		})
	}
}

func TestCheckAuthenticatedUser(t *testing.T) {
	gin.SetMode(gin.TestMode)

	payload := auth.JWTData{
		Email:  "test@test.com",
		Client: "test-client",
	}
	fakeUserPayload := auth.JWTData{
		Email:  "fake@test.com",
		Client: "test-client",
	}
	token, _ := auth.GenerateToken(1*time.Hour, payload, "test-secret")
	invalidToken, _ := auth.GenerateToken(1*time.Hour, payload, "invalid-secret")
	invalidUser, _ := auth.GenerateToken(1*time.Hour, fakeUserPayload, "test-secret")

	config := &apiConfig.Config{
		JWTSecret:          "test-secret",
		MongoDBURI:         "mongodb://localhost:27017",
		MongoDBName:        "conure-test",
		AuthStrategySystem: "local",
	}
	mongo, _ := database.ConnectToMongoDB(config.MongoDBURI, config.MongoDBName)
	defer cleanUpDB(mongo)

	user := auth.User{
		Email:  "test@test.com",
		Client: "test-client",
	}
	_ = user.Create(mongo)

	router := gin.New()
	router.Use(CheckAuthenticatedUser(config, mongo))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "success"})
	})

	tests := []struct {
		name         string
		auth         string
		expectedCode int
	}{
		{
			name:         "Valid token",
			auth:         "Bearer " + token,
			expectedCode: http.StatusOK,
		},
		{
			name:         "Invalid token",
			auth:         "Bearer " + invalidToken,
			expectedCode: http.StatusUnauthorized,
		},
		{
			name:         "Empty token",
			auth:         "",
			expectedCode: http.StatusUnauthorized,
		},
		{
			name:         "Invalid user",
			auth:         "Bearer " + invalidUser,
			expectedCode: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest("GET", "/test", nil)
			req.Header.Set("Authorization", tt.auth)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != tt.expectedCode {
				t.Errorf("Expected %v, got %v", tt.expectedCode, w.Code)
			}
		})
	}

	// test invalid strategy with one endpoint
	config.AuthStrategySystem = "fake-strategy"
	router = gin.New()
	router.Use(CheckAuthenticatedUser(config, mongo))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "success"})
	})

	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected %v, got %v", http.StatusUnauthorized, w.Code)
	}
}
