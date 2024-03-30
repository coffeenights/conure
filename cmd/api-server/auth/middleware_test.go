package auth

import (
	"github.com/coffeenights/conure/cmd/api-server/models"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apiConfig "github.com/coffeenights/conure/cmd/api-server/config"
	"github.com/coffeenights/conure/cmd/api-server/database"
)

func cleanUpDB(mongo *database.MongoDB) {
	err := mongo.Client.Database(mongo.DBName).Drop(nil)
	if err != nil {
		panic(err)
	}
}

func TestCheckCurrentUserValidToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	config := &apiConfig.Config{
		JWTSecret:   "test-secret",
		MongoDBURI:  "mongodb://localhost:27017",
		MongoDBName: "conure-test",
	}
	router := gin.New()

	mongo, _ := database.ConnectToMongoDB(config.MongoDBURI, config.MongoDBName)
	defer cleanUpDB(mongo)

	user := models.User{
		Email:  "test@test.com",
		Client: "test-client",
	}
	err := user.Create(mongo)
	require.NoError(t, err)

	router.Use(CheckCurrentUser(config, mongo))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "success"})
	})

	// Create a request with an invalid Authorization header
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code, "(invalid token) should return 401 Unauthorized")

	// Create a request with no Authorization header
	req, _ = http.NewRequest("GET", "/test", nil)

	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code, "(no token) should return 401 Unauthorized")

	// Create a request with an invalid format on Authorization header
	req, _ = http.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "invalid-token")

	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code, "(invalid format) should return 401 Unauthorized")

	// Create a request with an invalid format on Authorization header
	req, _ = http.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "InvalidFormat invalid-token")

	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code, "(invalid format) should return 401 Unauthorized")

	// Create a request with an unregistered user
	payload := JWTData{
		Email:  "fake-user@test.com",
		Client: user.Client,
	}
	token, err := GenerateToken(1*time.Hour, payload, config.JWTSecret)
	require.NoError(t, err)
	req, _ = http.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code, "(unregistered user ) should return 401 Unauthorized")

	// Create a request with a valid Authorization header
	payload.Email = user.Email
	token, err = GenerateToken(1*time.Hour, payload, config.JWTSecret)
	require.NoError(t, err)
	req, _ = http.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code, "(valid token) should return 200 OK")

}
