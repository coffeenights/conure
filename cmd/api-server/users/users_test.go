package users

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/joho/godotenv/autoload"
	"github.com/stretchr/testify/assert"

	"github.com/coffeenights/conure/cmd/api-server/auth"
	apiConfig "github.com/coffeenights/conure/cmd/api-server/config"
	"github.com/coffeenights/conure/cmd/api-server/database"
	"github.com/coffeenights/conure/cmd/api-server/models"
)

func cleanUpDB(mongo *database.MongoDB) {
	_ = mongo.Client.Database(mongo.DBName).Drop(context.Background())
}

func newTestRouter(t *testing.T) (*gin.Engine, *database.MongoDB, *apiConfig.Config) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	conf := &apiConfig.Config{
		JWTSecret:          "test-secret",
		MongoDBURI:         "mongodb://localhost:27017",
		MongoDBName:        "conure-test-users",
		AuthStrategySystem: "local",
	}
	mongo, err := database.ConnectToMongoDB(conf.MongoDBURI, conf.MongoDBName)
	if err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	handler := NewHandler(conf, mongo)
	GenerateRoutes("/users", router, handler)
	return router, mongo, conf
}

func makeUser(t *testing.T, mongo *database.MongoDB, email string, role models.Role) *models.User {
	t.Helper()
	hashed, _ := auth.GenerateFromPassword("Password123")
	u := &models.User{Email: email, Password: hashed, Role: role}
	if err := u.Create(mongo); err != nil {
		t.Fatalf("create user %s: %v", email, err)
	}
	return u
}

func tokenFor(t *testing.T, conf *apiConfig.Config, email string) string {
	t.Helper()
	tok, err := auth.GenerateToken(time.Hour, auth.JWTData{Email: email}, conf.JWTSecret)
	if err != nil {
		t.Fatal(err)
	}
	return tok
}

func TestUsers_DeveloperIsForbidden(t *testing.T) {
	router, mongo, conf := newTestRouter(t)
	defer cleanUpDB(mongo)

	dev := makeUser(t, mongo, "dev@test.io", models.RoleDeveloper)
	_ = dev

	req, _ := http.NewRequest(http.MethodGet, "/users/", nil)
	req.AddCookie(&http.Cookie{Name: "auth", Value: tokenFor(t, conf, "dev@test.io")})
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusForbidden, w.Code, "developer must be blocked from /users")
}

func TestUsers_AdminCRUD(t *testing.T) {
	router, mongo, conf := newTestRouter(t)
	defer cleanUpDB(mongo)

	admin := makeUser(t, mongo, "admin@test.io", models.RoleAdmin)
	cookie := &http.Cookie{Name: "auth", Value: tokenFor(t, conf, admin.Email)}

	// Create
	createBody, _ := json.Marshal(map[string]string{
		"email":    "new@test.io",
		"password": "Password123",
		"role":     "developer",
	})
	req, _ := http.NewRequest(http.MethodPost, "/users/", bytes.NewReader(createBody))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusCreated, w.Code, "admin must be able to create a user")
	var createResp UserResponse
	if err := json.Unmarshal(w.Body.Bytes(), &createResp); err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "new@test.io", createResp.Email)
	assert.Equal(t, models.RoleDeveloper, createResp.Role)
	newID := createResp.ID.Hex()

	// Get
	req, _ = http.NewRequest(http.MethodGet, "/users/"+newID, nil)
	req.AddCookie(cookie)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Update — promote to admin
	updateBody, _ := json.Marshal(map[string]string{"role": "admin"})
	req, _ = http.NewRequest(http.MethodPatch, "/users/"+newID, bytes.NewReader(updateBody))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	var updateResp UserResponse
	_ = json.Unmarshal(w.Body.Bytes(), &updateResp)
	assert.Equal(t, models.RoleAdmin, updateResp.Role)

	// Reset password — random
	req, _ = http.NewRequest(http.MethodPost, "/users/"+newID+"/reset-password", bytes.NewReader([]byte("{}")))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	var resetResp ResetPasswordResponse
	_ = json.Unmarshal(w.Body.Bytes(), &resetResp)
	assert.NotEmpty(t, resetResp.Password, "reset must return the new password")

	// Delete
	req, _ = http.NewRequest(http.MethodDelete, "/users/"+newID, nil)
	req.AddCookie(cookie)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Get after delete -> not found
	req, _ = http.NewRequest(http.MethodGet, "/users/"+newID, nil)
	req.AddCookie(cookie)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestUsers_AdminCannotDeleteSelf(t *testing.T) {
	router, mongo, conf := newTestRouter(t)
	defer cleanUpDB(mongo)

	admin := makeUser(t, mongo, "admin@test.io", models.RoleAdmin)
	cookie := &http.Cookie{Name: "auth", Value: tokenFor(t, conf, admin.Email)}

	req, _ := http.NewRequest(http.MethodDelete, "/users/"+admin.ID.Hex(), nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusForbidden, w.Code, "admin must not delete themselves")
}
